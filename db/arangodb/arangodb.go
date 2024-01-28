package arangodb

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/arangodb/go-driver/jwt"
	"github.com/google/uuid"
	"github.com/kylelemons/godebug/pretty"
	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"golang.org/x/crypto/bcrypt"
)

const (
	GRAPH_DB_NAME = `learngraph`

	COLLECTION_NODES     = `nodes`
	COLLECTION_EDGES     = `edges`
	COLLECTION_USERS     = `users`
	COLLECTION_NODEEDITS = `nodeedits`
	COLLECTION_EDGEEDITS = `edgeedits`

	INDEX_HASH_USER_EMAIL    = "User_EMail"
	INDEX_HASH_USER_USERNAME = "User_Username"

	AUTHENTICATION_TOKEN_EXPIRY = 12 * 30 * 24 * time.Hour // ~ 1 year
	MIN_PASSWORD_LENGTH         = 10
	MIN_USERNAME_LENGTH         = 4
)

//go:generate mockgen -destination arangodboperations_mock.go -package arangodb . ArangoDBOperations
type ArangoDBOperations interface {
	Init(conf db.Config) (db.DB, error)
	OpenDatabase(ctx context.Context) error
	CreateDBWithSchema(ctx context.Context) error
	ValidateSchema(ctx context.Context) (SchemaUpdateAction, error)
	CollectionsExist(ctx context.Context) (bool, error)
	AddNodeToEditNode(ctx context.Context) error
}

// FIXME(skep): every public interface of ArangoDB must call EnsureSchema in
// case this call is the first after startup - how to ensure this for future
// interfaces?
// Note: A good alternative would be to apply a circuit-breaker inside the
//		 resolver before the DB-call, to ensure DB availability.

// implements db.DB
type ArangoDB struct {
	conn    driver.Connection
	cli     driver.Client
	db      driver.Database
	timeNow func() time.Time
}

func QueryReadAll[T any](ctx context.Context, adb *ArangoDB, query string, bindVars ...map[string]interface{}) ([]T, error) {
	ctx = driver.WithQueryCount(ctx, true) // needed to call .Count() on the cursor below
	var (
		cursor driver.Cursor
		err    error
	)
	if len(bindVars) == 1 {
		cursor, err = adb.db.Query(ctx, query, bindVars[0])
	} else {
		cursor, err = adb.db.Query(ctx, query, nil)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "query '%s' failed", query)
	}

	out := make([]T, cursor.Count())
	for i := int64(0); i < cursor.Count(); i++ {
		t := new(T)
		meta, err := cursor.ReadDocument(ctx, t)
		out[i] = *t
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read document: %v", meta)
		}
	}

	return out, nil
}

func (adb *ArangoDB) Graph(ctx context.Context) (*model.Graph, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := QueryReadAll[db.Node](ctx, adb, `FOR n in nodes RETURN n`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query nodes")
	}

	edges, err := QueryReadAll[db.Edge](ctx, adb, `FOR e in edges RETURN e`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query edges")
	}

	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).Graph(nodes, edges), nil
}

// XXX: how to test if begin/end transaction lock the right databases?
func (adb *ArangoDB) beginTransaction(ctx context.Context, cols driver.TransactionCollections) (driver.TransactionID, error) {
	stamp, ok := ctx.Deadline()
	var opts *driver.BeginTransactionOptions
	if ok {
		opts = &driver.BeginTransactionOptions{LockTimeout: adb.timeNow().Sub(stamp)}
	}
	return adb.db.BeginTransaction(ctx, cols, opts)
}

func (adb *ArangoDB) endTransaction(ctx context.Context, transaction driver.TransactionID, err *error) {
	if *err != nil {
		*err = adb.db.AbortTransaction(ctx, transaction, nil)
	} else {
		*err = adb.db.CommitTransaction(ctx, transaction, nil)
	}
}

func (adb *ArangoDB) CreateNode(ctx context.Context, user db.User, description *model.Text, resources *model.Text) (string, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return "", err
	}
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_NODES, COLLECTION_NODEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	col, err := adb.db.Collection(ctx, COLLECTION_NODES)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODES)
	}
	node := db.Node{
		Description: ConvertToDBText(description),
	}
	if resources != nil {
		node.Resources = ConvertToDBText(resources)
	}
	meta, err := col.CreateDocument(ctx, node)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create node '%v', meta: '%v'", node, meta)
	}
	node.Key = meta.ID.Key()
	col, err = adb.db.Collection(ctx, COLLECTION_NODEEDITS)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODEEDITS)
	}
	nodeedit := db.NodeEdit{
		Node:      node.Key,
		User:      user.Key,
		Type:      db.NodeEditTypeCreate,
		NewNode:   node,
		CreatedAt: adb.timeNow().UnixMilli(),
	}
	meta, err = col.CreateDocument(ctx, nodeedit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create nodeedit '%v', meta: '%v'", nodeedit, meta)
	}
	return node.Key, err
}

// CreateEdge creates an edge from node `from` to node `to` with weight `weight`.
// Nodes use ArangoDB format <collection>/<nodeID>.
// Returns the ID of the created edge and nil on success. On failure an empty
// string and an error is returned.
func (adb *ArangoDB) CreateEdge(ctx context.Context, user db.User, from, to string, weight float64) (string, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return "", err
	}
	if from == to {
		return "", errors.Errorf("no self-linking nodes allowed (from == to == '%s')", from)
	}
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Read: []string{COLLECTION_NODES}, Write: []string{COLLECTION_EDGES, COLLECTION_EDGEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	col, err := adb.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	edges, err := QueryReadAll[db.Edge](ctx, adb, `FOR e in edges FILTER e._from == @from AND e._to == @to RETURN e`, map[string]interface{}{
		"from": from, "to": to,
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to query duplicate edges (%v)", edges)
	}
	if len(edges) > 0 {
		return "", errors.Errorf("edge already exists: %v", edges)
	}
	edge := db.Edge{
		From:   from,
		To:     to,
		Weight: weight,
	}
	if err := adb.nodesExist(ctx, []string{from, to}); err != nil {
		return "", err
	}
	meta, err := col.CreateDocument(ctx, &edge)
	edge.Key = meta.ID.Key()
	if err != nil {
		return "", errors.Wrapf(err, "failed to create edge '%v', meta: '%v'", edge, meta)
	}
	col, err = adb.db.Collection(ctx, COLLECTION_EDGEEDITS)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGEEDITS)
	}
	edit := &db.EdgeEdit{
		Edge:      edge.Key,
		User:      user.Key,
		Type:      db.EdgeEditTypeCreate,
		Weight:    weight,
		CreatedAt: adb.timeNow().UnixMilli(),
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create db.EdgeEdit '%#v', meta: '%v'", edit, meta)
	}
	return edge.Key, err
}

// nodesExist returns nil if all nodes exist, otherwise on the first
// non-existing node an error is returned
func (adb *ArangoDB) nodesExist(ctx context.Context, nodesWithCollection []string) error {
	for _, node := range nodesWithCollection {
		if err := adb.nodeExists(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// nodeExists takes a nodeWithCollection in arangoDB format
// <collection>/<nodeID>, returns an error if the node (or collection) does not
// exist
func (adb *ArangoDB) nodeExists(ctx context.Context, nodeWithCollection string) error {
	tmp := strings.Split(nodeWithCollection, "/")
	if len(tmp) != 2 {
		return errors.New("internal error: node format invalid")
	}
	collection_name, node_name := tmp[0], tmp[1]
	collection, err := adb.db.Collection(ctx, collection_name)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", collection_name)
	}
	if exists, err := collection.DocumentExists(ctx, node_name); err != nil || !exists {
		if err != nil {
			return errors.Wrapf(err, "cannot create edge: node existance check failed for '%s': '%v'", node_name, err)
		}
		return errors.Errorf("cannot create edge: node '%s' does not exist", node_name)
	}
	return nil
}

func (adb *ArangoDB) EditNode(ctx context.Context, user db.User, nodeID string, description *model.Text, resources *model.Text) error {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return err
	}
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_NODES, COLLECTION_NODEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	col, err := adb.db.Collection(ctx, COLLECTION_NODES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODES)
	}
	node := db.Node{}
	// Note: currently it is not required to load the old data, since nodes
	// only have the describtion attribute, which is merged on db.DB level
	meta, err := col.ReadDocument(ctx, nodeID, &node)
	if err != nil {
		return errors.Wrapf(err, "failed to read node id = %s, meta: '%v'", nodeID, meta)
	}
	node.Description = ConvertToDBText(description)
	if resources != nil {
		node.Resources = ConvertToDBText(resources)
	}
	// merged on db level
	//node.Description = MergeText(node.Description, ConvertToDBText(description))
	meta, err = col.UpdateDocument(ctx, nodeID, &node)
	if err != nil {
		return errors.Wrapf(err, "failed to update node id = %s, node: %v, meta: '%v'", nodeID, node, meta)
	}
	// read it back to get the merged node description for usage in the NodeEdit query below
	meta, err = col.ReadDocument(ctx, nodeID, &node)
	if err != nil {
		return errors.Wrapf(err, "failed to read edited node id = %s, node: %v, meta: '%v'", nodeID, node, meta)
	}
	col, err = adb.db.Collection(ctx, COLLECTION_NODEEDITS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODEEDITS)
	}
	edit := db.NodeEdit{
		Node:      nodeID,
		User:      user.Key,
		Type:      db.NodeEditTypeEdit,
		NewNode:   node,
		CreatedAt: adb.timeNow().UnixMilli(),
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return errors.Wrapf(err, "failed to create db.NodeEdit{%v}", edit)
	}
	return err
}

func (adb *ArangoDB) AddEdgeWeightVote(ctx context.Context, user db.User, edgeID string, weight float64) error {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return err
	}
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_EDGES, COLLECTION_EDGEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	col, err := adb.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	edge := db.Edge{}
	meta, err := col.ReadDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to read edge: %v", meta)
	}
	// TODO(skep): should move aggregation to separate module/application
	edits, err := QueryReadAll[db.EdgeEdit](ctx, adb, `FOR edit in edgeedits FILTER edit.edge == @edge AND edit.weight != 0 RETURN edit`, map[string]interface{}{
		"edge": edgeID,
	})
	sum := db.Sum(edits, func(edit db.EdgeEdit) float64 { return edit.Weight })
	averageWeight := (sum + weight) / float64(len(edits)+1)
	edge.Weight = averageWeight
	meta, err = col.UpdateDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to update edge: %v\nedge: %v", meta, edge)
	}
	col, err = adb.db.Collection(ctx, COLLECTION_EDGEEDITS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGEEDITS)
	}
	edit := db.EdgeEdit{
		User:      user.Key,
		Edge:      edge.Key,
		Type:      db.EdgeEditTypeVote,
		Weight:    weight,
		CreatedAt: adb.timeNow().UnixMilli(),
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return errors.Wrapf(err, "failed to create db.EdgeEdit{%v}: %v", edit, meta)
	}
	return err
}

func NewArangoDB(conf db.Config) (db.DB, error) {
	db := ArangoDB{
		timeNow: time.Now,
	}
	return db.Init(conf)
}

func ReadSecretFile(file string) (string, error) {
	tmp, err := os.ReadFile(file)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read JWT secret from file '%s'", file)
	}
	if len(tmp) == 0 {
		return "", errors.Errorf("JWT secret file '%s' is empty", file)
	}
	return strings.TrimRight(string(tmp), "\n"), nil
}

func GetAuthentication(conf db.Config) (driver.Authentication, error) {
	if conf.NoAuthentication {
		return driver.RawAuthentication(""), nil
	}
	if conf.JwtToken != "" {
		hdr := fmt.Sprintf("bearer %s", conf.JwtToken)
		return driver.RawAuthentication(hdr), nil
	}
	if conf.JwtSecretPath != "" {
		secret, err := ReadSecretFile(conf.JwtSecretPath)
		if err != nil {
			return nil, err
		}
		hdr, err := jwt.CreateArangodJwtAuthorizationHeader(secret, "learngraph-backend")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create JWT authorization header")
		}
		return driver.RawAuthentication(hdr), nil
	}
	return nil, errors.New("no authentication available")
}

func (adb *ArangoDB) Init(conf db.Config) (db.DB, error) {
	var err error
	adb.conn, err = http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{conf.Host},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to arangodb instance")
	}
	auth, err := GetAuthentication(conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get authentication data")
	}
	adb.cli, err = driver.NewClient(driver.ClientConfig{
		Connection:     adb.conn,
		Authentication: auth,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arangodb client")
	}
	return adb, nil
}

func (adb *ArangoDB) OpenDatabase(ctx context.Context) error {
	var err error
	if adb.db != nil {
		return nil
	}
	adb.db, err = adb.cli.Database(ctx, GRAPH_DB_NAME)
	if err != nil {
		return errors.Wrapf(err, "failed to open database '%s'", GRAPH_DB_NAME)
	}
	return nil
}

var AQL_SCHEMA_VALIDATE = `
let schema = SCHEMA_GET(@collection)
RETURN false NOT IN ( for o in @@collection return SCHEMA_VALIDATE(o, schema).valid)
`

func (adb *ArangoDB) validateSchemaForCollection(ctx context.Context, collection string, opts *driver.CollectionSchemaOptions) (SchemaUpdateAction, error) {
	col, err := adb.db.Collection(ctx, collection)
	if err != nil {
		return SchemaUnchanged, errors.Wrapf(err, "failed to access '%s' collection", collection)
	}
	props, err := col.Properties(ctx)
	if err != nil {
		return SchemaUnchanged, errors.Wrapf(err, "failed to access '%s' collection properties", collection)
	}
	diff := pretty.Compare(props.Schema, opts) // FIXME(skep): see DeepEqual problem in schema.go
	//if reflect.DeepEqual(props.Schema, opts) {
	if diff == "" {
		return SchemaUnchanged, nil
	}
	//else {
	//	fmt.Printf("diff:\n%s\n", diff)
	//}
	if collection == COLLECTION_NODEEDITS {
		// XXX: bad hack for validation fails due to missing created_at
		// property, somehow it only showed once changing schema twice, not on
		// the change that caused the invalid-schema error?!
		_, err := QueryReadAll[bool](ctx, adb, `FOR e IN nodeedits
			FILTER NOT HAS(e, "created_at")
			UPDATE { _key: e._key, created_at: 0 } in nodeedits
		`)
		if err != nil {
			return SchemaChangedButNoActionRequired, errors.Wrap(err, "failed to add 'created_at' field")
		}
	}
	// You wonder why @collection & @@collection? See
	// https://www.arangodb.com/docs/stable/aql/fundamentals-bind-parameters.html#syntax
	valid, err := QueryReadAll[bool](ctx, adb, AQL_SCHEMA_VALIDATE, map[string]interface{}{
		"@collection": collection,
		"collection":  collection,
	})
	if err != nil {
		return SchemaChangedButNoActionRequired, errors.Wrapf(err, "failed to execute AQL: %v", AQL_SCHEMA_VALIDATE)
	}
	if len(valid) != 1 {
		return SchemaChangedButNoActionRequired, errors.Errorf("unknown AQL return value\ncurrent/old schema:\n%#v\nnew schema:\n%#v", props.Schema, opts)
	}
	if valid[0] == false {
		//		res, _ := QueryReadAll[map[string]interface{}](ctx, adb, `
		//let schema = SCHEMA_GET(@collection)
		//FOR o IN @@collection
		//	FILTER SCHEMA_VALIDATE(o, schema).valid == false
		//	LIMIT 1
		//	RETURN o
		//` , map[string]interface{}{
		//			"@collection": collection,
		//			"collection":  collection,
		//		})
		//		log.Ctx(ctx).Error().Msgf("offending sample object in collection %s:\n%v", collection, res)
		return SchemaChangedButNoActionRequired, errors.Errorf("incompatible schemas!\ncurrent/old schema:\n%#v\nnew schema:\n%#v", props.Schema, opts)
	}
	err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: opts})
	if err != nil {
		return SchemaChangedButNoActionRequired, errors.Wrapf(err, "failed to set schema options (to collection %s): %v", collection, opts)
	}
	if collection == COLLECTION_NODEEDITS {
		if _, exists := props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{})["newnode"]; !exists {
			return SchemaChangedAddNodeToEditNode, nil
		}
	}
	return SchemaChangedButNoActionRequired, nil
}

// returns true, if schema changed, false otherwise
type SchemaUpdateAction string

const (
	SchemaUnchanged                  SchemaUpdateAction = "unchanged"
	SchemaChangedButNoActionRequired SchemaUpdateAction = "changed-but-no-action-required"
	SchemaChangedAddNodeToEditNode   SchemaUpdateAction = "changed-add-node-to-editnode"
)

func (adb *ArangoDB) ValidateSchema(ctx context.Context) (SchemaUpdateAction, error) {
	action := SchemaUnchanged
	for _, collection := range CollectionSpecification {
		newaction, err := adb.validateSchemaForCollection(ctx, collection.Name, collection.Options.Schema)
		if action == SchemaUnchanged {
			action = newaction
		}
		if err != nil {
			return action, errors.Wrapf(err, "validate schema for '%s' failed", collection.Name)
		}
	}
	return action, nil
}

func (adb *ArangoDB) AddNodeToEditNode(ctx context.Context) error {
	updateQuery := fmt.Sprintf(`
		FOR edit IN %s
			FILTER edit.newnode._key == null
			UPDATE edit WITH { newnode: DOCUMENT("%s", edit.node) } IN %s
	`, COLLECTION_NODEEDITS, COLLECTION_NODES, COLLECTION_NODEEDITS)
	_, err := adb.db.Query(ctx, updateQuery, nil)
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", updateQuery)
	}
	return nil
}

func (adb *ArangoDB) CollectionsExist(ctx context.Context) (bool, error) {
	for _, col := range CollectionSpecification {
		exists, err := adb.db.CollectionExists(ctx, col.Name)
		if err != nil {
			return exists, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func (adb *ArangoDB) CreateDBWithSchema(ctx context.Context) error {
	exists, err := adb.cli.DatabaseExists(ctx, GRAPH_DB_NAME)
	if err != nil {
		return errors.Wrapf(err, "failed to check db.DB existence `%v`: %v", GRAPH_DB_NAME, adb)
	}
	var learngraphDB driver.Database
	if !exists {
		learngraphDB, err = adb.cli.CreateDatabase(ctx, GRAPH_DB_NAME, nil) //&driver.CreateDatabaseOptions{})
	} else {
		learngraphDB, err = adb.cli.Database(ctx, GRAPH_DB_NAME)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to create/open db.DB `%v`: %v", GRAPH_DB_NAME, adb)
	}
	adb.db = learngraphDB

	for _, collection := range CollectionSpecification {
		if exists, err = adb.db.CollectionExists(ctx, collection.Name); !exists || err != nil {
			col, err := adb.db.CreateCollection(ctx, collection.Name, &collection.Options)
			if err != nil {
				return errors.Wrapf(err, "failed to create '%s' collection", collection.Name)
			}
			for _, index := range collection.Indexes {
				_, _, err = col.EnsurePersistentIndex(ctx, []string{index.Property}, &driver.EnsurePersistentIndexOptions{
					Unique: true, Sparse: true, Name: index.Name,
				})
				if err != nil {
					return errors.Wrapf(err, "failed to create index '%s' on collection '%s'", index.Name, collection.Name)
				}
			}
		}
	}

	if exists, err = adb.db.GraphExists(ctx, "graph"); !exists || err != nil {
		_, err = adb.db.CreateGraph(ctx, "graph", &driver.CreateGraphOptions{
			EdgeDefinitions: []driver.EdgeDefinition{
				{
					Collection: COLLECTION_EDGES,
					To:         []string{COLLECTION_NODES},
					From:       []string{COLLECTION_NODES},
				},
			},
		})
		if err != nil {
			return errors.Wrap(err, "failed to create graph")
		}
	}

	return nil
}

func QueryExists(ctx context.Context, adb *ArangoDB, collection, property, value string) (bool, error) {
	existsQuery := fmt.Sprintf(`RETURN LENGTH(for u in %s FILTER u.%s == @%s LIMIT 1 RETURN u) > 0`, collection, property, property)
	cursor, err := adb.db.Query(ctx, existsQuery, map[string]interface{}{
		property: value,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query existance with '%s' for %s '%s'", existsQuery, property, value)
	}
	var exists bool
	_, err = cursor.ReadDocument(ctx, &exists)
	if err != nil {
		return false, errors.Wrap(err, "failed to read from cursor")
	}
	return exists, nil
}

// VerifyUserInput returns a CreateUserResult with an error message on
// *invalid* user input, on valid user input nil is returned.
func VerifyUserInput(ctx context.Context, user db.User, password string) (*model.CreateUserResult) {
	if len(password) < MIN_PASSWORD_LENGTH {
		msg := fmt.Sprintf("Password must be at least length %d, the provided one has only %d characters.", MIN_PASSWORD_LENGTH, len(password))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if len(user.Username) < MIN_USERNAME_LENGTH {
		msg := fmt.Sprintf("Username must be at least length %d, the provided one has only %d characters.", MIN_USERNAME_LENGTH, len(user.Username))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if _, err := mail.ParseAddress(user.EMail); err != nil {
		msg := fmt.Sprintf("Invalid EMail: '%s'", user.EMail)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	return nil
}

func (adb *ArangoDB) verifyUserInput(ctx context.Context, user db.User, password string) (*model.CreateUserResult, error) {
	if res := VerifyUserInput(ctx, user, password); res != nil {
		return res, nil
	}
	if userExists, err := QueryExists(ctx, adb, COLLECTION_USERS, "username", user.Username); err != nil || userExists {
		msg := fmt.Sprintf("Username already exists: '%s'", user.Username)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, err
	}
	if emailExists, err := QueryExists(ctx, adb, COLLECTION_USERS, "email", user.EMail); err != nil || emailExists {
		msg := fmt.Sprintf("EMail already exists: '%s'", user.EMail)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, err
	}
	return nil, nil
}

func (adb *ArangoDB) makeNewAuthenticationToken() db.AuthenticationToken {
	return db.AuthenticationToken{
		Token:  uuid.New().String(), // TODO(skep): use jwt + .. HMAC? remember https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/
		Expiry: adb.timeNow().Add(AUTHENTICATION_TOKEN_EXPIRY).UnixMilli(),
	}
}

func (adb *ArangoDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return nil, err
	}
	user := db.User{
		Username: username,
		EMail:    email,
	}
	invalidInput, err := adb.verifyUserInput(ctx, user, password)
	if err != nil {
		return nil, err
	}
	if invalidInput != nil {
		return invalidInput, nil
	}
	return adb.createUser(ctx, user, password)
}

func (adb *ArangoDB) createUser(ctx context.Context, user db.User, password string) (*model.CreateUserResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create password hash for user '%v'", user)
	}
	user.PasswordHash = string(hash)
	user.Tokens = []db.AuthenticationToken{adb.makeNewAuthenticationToken()}
	col, err := adb.db.Collection(ctx, COLLECTION_USERS)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_USERS)
	}
	meta, err := col.CreateDocument(ctx, user)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create user '%#v', meta: '%v'", user, meta)
	}
	return &model.CreateUserResult{
		Login: &model.LoginResult{
			Success:  true,
			Token:    user.Tokens[0].Token,
			UserID:   meta.ID.Key(),
			UserName: user.Username,
		},
	}, nil
}

func (adb *ArangoDB) Login(ctx context.Context, auth model.LoginAuthentication) (*model.LoginResult, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return nil, err
	}
	user, err := adb.getUserByProperty(ctx, "email", auth.Email)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user")
	}
	if user == nil {
		msg := "User does not exist"
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(auth.Password)); err != nil {
		msg := "Password missmatch"
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
	}

	newToken := adb.makeNewAuthenticationToken()
	user.Tokens = append(user.Tokens, newToken)
	updateQuery := `UPDATE { _key: @userkey, authenticationtokens: @authtokens } IN users`
	_, err = adb.db.Query(ctx, updateQuery, map[string]interface{}{
		"userkey":    user.Key,
		"authtokens": user.Tokens,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "query '%s' failed", updateQuery)
	}

	return &model.LoginResult{
		Success:  true,
		Token:    newToken.Token,
		UserID:   user.Document.Key,
		UserName: user.Username,
	}, nil
}

// returns the user with given username, if no such user exists, returns (nil, nil)
func (adb *ArangoDB) getUserByProperty(ctx context.Context, property, value string) (*db.User, error) {
	query := fmt.Sprintf("FOR u in users FILTER u.%s == @%s RETURN u", property, property)
	users, err := QueryReadAll[db.User](ctx, adb, query,
		map[string]interface{}{property: value})
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving user with %s='%s'", property, value)
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (adb *ArangoDB) deleteUserByKey(ctx context.Context, key string) error {
	user, err := adb.getUserByProperty(ctx, "_key", key)
	if err != nil {
		return errors.Wrapf(err, "failed to get user by _key='%s'", key)
	}
	if user == nil {
		return errors.Errorf("no user with _key='%s' exists", key)
	}
	if db.FindFirst(user.Tokens, adb.isValidToken(middleware.CtxGetAuthentication(ctx))) == nil {
		return errors.Errorf("not authenticated to delete user key='%s'", key)
	}
	col, err := adb.db.Collection(ctx, COLLECTION_USERS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_USERS)
	}
	meta, err := col.RemoveDocument(ctx, key)
	if err != nil {
		return errors.Wrapf(err, "failed to remove user with key='%s', meta=%v", key, meta)
	}
	return nil
}

// deletes the account identified by username, this requires a valid
// authentication token passed via the context
func (adb *ArangoDB) DeleteAccount(ctx context.Context) error {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return err
	}
	key := middleware.CtxGetUserID(ctx)
	if key == "" {
		return errors.New("no userID in HTTP-header found")
	}
	return adb.deleteUserByKey(ctx, key)
}

func (adb *ArangoDB) Logout(ctx context.Context) error {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return err
	}

	key := middleware.CtxGetUserID(ctx)
	if key == "" {
		return errors.New("missing userID in HTTP-header: cannot logout")
	}
	user, err := adb.getUserByProperty(ctx, "_key", key)
	if err != nil {
		return errors.Wrapf(err, "failed to query user via userID '%s'", key)
	}
	if user == nil {
		return errors.Errorf("no user found with userID '%s'", key)
	}
	activeTokens := user.Tokens
	token := middleware.CtxGetAuthentication(ctx)
	if !db.ContainsP(activeTokens, token, accessAuthTokenString) {
		return errors.Errorf("not authenticated to logout user key='%s'", key)
	}
	user.Tokens = db.RemoveIf(activeTokens, func(t db.AuthenticationToken) bool { return t.Token == token })
	updateQuery := `UPDATE { _key: @userkey, authenticationtokens: @authtokens } IN users`
	_, err = adb.db.Query(ctx, updateQuery, map[string]interface{}{
		"userkey":    user.Key,
		"authtokens": user.Tokens,
	})
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", updateQuery)
	}
	return nil
}

func accessAuthTokenString(t db.AuthenticationToken) string { return t.Token }

func (adb *ArangoDB) getAuthenticatedUser(ctx context.Context) (*db.User, error) {
	id, token := middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx)
	user, err := adb.getUserByProperty(ctx, "_key", id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user")
	}
	if user == nil {
		return nil, nil // user does not exist
	}
	if db.FindFirst(user.Tokens, adb.isValidToken(token)) == nil {
		return nil, nil // no matching & valid token
	}
	return user, nil
}

func (adb *ArangoDB) IsUserAuthenticated(ctx context.Context) (bool, *db.User, error) {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return false, nil, err
	}
	user, err := adb.getAuthenticatedUser(ctx)
	return user != nil, user, err
}

func (adb *ArangoDB) isValidToken(token string) func(t db.AuthenticationToken) bool {
	return func(t db.AuthenticationToken) bool {
		if t.Token == token && time.UnixMilli(t.Expiry).After(adb.timeNow()) {
			return true
		}
		return false
	}
}

func (adb *ArangoDB) DeleteAccountWithData(ctx context.Context, username, adminkey string) error {
	err := EnsureSchema(adb, ctx)
	if err != nil {
		return err
	}
	targetUser, err := adb.getUserByProperty(ctx, "username", username)
	if err != nil {
		return errors.Wrapf(err, "failed to get user by name '%s'", username)
	}
	if targetUser == nil {
		return errors.Errorf("user with name '%s' does not exists", username)
	}
	currentUser, err := adb.getAuthenticatedUser(ctx)
	if err != nil {
		return err
	}
	if currentUser == nil {
		return errors.Errorf("userID='%s' is not authenticated / non-existent", middleware.CtxGetUserID(ctx))
	}
	if !db.Contains(currentUser.Roles, db.RoleAdmin) {
		return errors.Errorf("user '%s' has does not have role '%s'", username, db.RoleAdmin)
	}
	col, err := adb.db.Collection(ctx, COLLECTION_USERS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_USERS)
	}
	meta, err := col.RemoveDocument(ctx, targetUser.Key)
	if err != nil {
		return errors.Wrapf(err, "failed to remove user with key='%s', meta=%v", targetUser.Key, meta)
	}
	return nil
}

func QueryBool(ctx context.Context, adb *ArangoDB, query string) (bool, error) {
	cursor, err := adb.db.Query(ctx, query, nil)
	if err != nil {
		return false, errors.Wrapf(err, "query '%s' failed", query)
	}
	var result bool
	_, err = cursor.ReadDocument(ctx, &result)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read from cursor")
	}
	return result, nil
}

func (adb *ArangoDB) DeleteNode(ctx context.Context, user db.User, ID string) error {
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_NODES, COLLECTION_NODEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	// check for edits from other users
	hasOtherUserEditsQuery := fmt.Sprintf(`
		RETURN LENGTH(
			FOR edit IN %s
			FILTER edit.node == "%s" AND edit.user != "%s"
			RETURN edit) > 0
	`, COLLECTION_NODEEDITS, ID, user.Key)
	hasOtherUserEdits, err := QueryBool(ctx, adb, hasOtherUserEditsQuery)
	if err != nil {
		return errors.Wrap(err, "failed to query other node-edits")
	}
	if hasOtherUserEdits {
		return errors.New("node has edits from other users, won't delete")
	}
	// check for edges to/from this node
	hasEdgesQuery := fmt.Sprintf(`
		RETURN LENGTH(
			FOR edge IN %s
			FILTER edge._from == "%s/%s" OR edge._to == "%s/%s"
			RETURN edge) > 0
	`, COLLECTION_EDGES, COLLECTION_NODES, ID, COLLECTION_NODES, ID)
	hasEdges, err := QueryBool(ctx, adb, hasEdgesQuery)
	if err != nil {
		return errors.Wrap(err, "failed to query edges to node")
	}
	if hasEdges {
		return errors.New("cannot delete node with edges, remove edges first")
	}
	// remove node & edits
	col, err := adb.db.Collection(ctx, COLLECTION_NODES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODES)
	}
	meta, err := col.RemoveDocument(ctx, ID)
	if err != nil {
		return errors.Wrapf(err, "failed to remove node ID='%s', meta=%v", ID, meta)
	}
	removeNodeEdits := fmt.Sprintf(`
		FOR edit IN %s
			FILTER edit.node == "%s"
			REMOVE edit IN %s
	`, COLLECTION_NODEEDITS, ID, COLLECTION_NODEEDITS)
	_, err = adb.db.Query(ctx, removeNodeEdits, nil)
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", removeNodeEdits)
	}
	return nil
}

func (adb *ArangoDB) DeleteEdge(ctx context.Context, user db.User, ID string) error {
	transaction, err := adb.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_EDGES, COLLECTION_EDGEEDITS}})
	defer adb.endTransaction(ctx, transaction, &err)
	// check for edits from other users
	hasEdgeOtherUserEdits := fmt.Sprintf(`
		RETURN LENGTH(
			FOR edit IN %s
			FILTER edit.edge == "%s" AND edit.user != "%s"
			RETURN edit) > 0
	`, COLLECTION_EDGEEDITS, ID, user.Key)
	cursor, err := adb.db.Query(ctx, hasEdgeOtherUserEdits, nil)
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", hasEdgeOtherUserEdits)
	}
	var hasOtherEdits bool
	_, err = cursor.ReadDocument(ctx, &hasOtherEdits)
	if err != nil {
		return errors.Wrapf(err, "failed to read from cursor")
	}
	if hasOtherEdits {
		return errors.New("edge has edits from other users, won't delete")
	}
	// remove edge & edits
	col, err := adb.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	meta, err := col.RemoveDocument(ctx, ID)
	if err != nil {
		return errors.Wrapf(err, "failed to remove edge ID='%s', meta=%v", ID, meta)
	}
	removeEdgeEdits := fmt.Sprintf(`
		FOR edit IN %s
			FILTER edit.edge == "%s"
			REMOVE edit IN %s
	`, COLLECTION_EDGEEDITS, ID, COLLECTION_EDGEEDITS)
	_, err = adb.db.Query(ctx, removeEdgeEdits, nil)
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", removeEdgeEdits)
	}
	return nil
}

func (adb *ArangoDB) Node(ctx context.Context, ID string) (*model.Node, error) {
	nodes, err := QueryReadAll[db.Node](ctx, adb, `FOR n in nodes FILTER n._key == @node RETURN n`, map[string]interface{}{
		"node": ID,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query node with ID='%s'", ID)
	}
	if len(nodes) != 1 {
		return nil, errors.Errorf("no node with ID='%s' found", ID)
	}
	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).Node(nodes[0]), nil
}
