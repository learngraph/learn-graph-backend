package db

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

	AUTHENTICATION_TOKEN_EXPIRY = 30 * 24 * time.Hour
	MIN_PASSWORD_LENGTH         = 10
	MIN_USERNAME_LENGTH         = 4
)

//go:generate mockgen -destination arangodboperations_mock.go -package db . ArangoDBOperations
type ArangoDBOperations interface {
	Init(conf Config) (DB, error)
	OpenDatabase(ctx context.Context) error
	CreateDBWithSchema(ctx context.Context) error
	ValidateSchema(ctx context.Context) (bool, error)
	CollectionsExist(ctx context.Context) (bool, error)
}

// FIXME(skep): every public interface of ArangoDB must call EnsureSchema in
// case this call is the first after startup - how to ensure this for future
// interfaces?
// Note: A good alternative would be to apply a circuit-breaker inside the
//		 resolver before the DB-call, to ensure DB availability.

// implements db.DB
type ArangoDB struct {
	conn driver.Connection
	cli  driver.Client
	db   driver.Database
}

// arangoDB document collection
type Document struct {
	Key string `json:"_key,omitempty"`
}

type Node struct {
	Document
	Description Text `json:"description"`
}

type NodeEdit struct {
	Document
	Node string       `json:"node"`
	User string       `json:"user"`
	Type NodeEditType `json:"type"`
}

type NodeEditType string

const (
	NodeEditTypeCreate NodeEditType = "create"
	NodeEditTypeEdit   NodeEditType = "edit"
)

type EdgeEdit struct {
	Document
	Edge   string       `json:"edge"`
	User   string       `json:"user"`
	Type   EdgeEditType `json:"type"`
	Weight float64      `json:"weight"`
}

type EdgeEditType string

const (
	EdgeEditTypeCreate EdgeEditType = "create"
	EdgeEditTypeVote   EdgeEditType = "edit"
)

// arangoDB edge collection, with custom additional fields
type Edge struct {
	Document
	From   string  `json:"_from"`
	To     string  `json:"_to"`
	Weight float64 `json:"weight"`
}

type User struct {
	Document
	Username     string                `json:"username"`
	PasswordHash string                `json:"passwordhash"`
	EMail        string                `json:"email"`
	Tokens       []AuthenticationToken `json:"authenticationtokens,omitempty"`
	Roles        []RoleType            `json:"roles,omitempty"`
}

type RoleType string

const (
	RoleAdmin RoleType = "admin"
)

type AuthenticationToken struct {
	Token string `json:"token"`
	// A unix time stamp in millisecond precision,
	// see https://docs.arangodb.com/3.11/aql/functions/date/#working-with-dates-and-indices
	Expiry int64 `json:"expiry"`
}

type Text map[string]string

func QueryReadAll[T any](ctx context.Context, db *ArangoDB, query string, bindVars ...map[string]interface{}) ([]T, error) {
	ctx = driver.WithQueryCount(ctx, true) // needed to call .Count() on the cursor below
	var (
		cursor driver.Cursor
		err    error
	)
	if len(bindVars) == 1 {
		cursor, err = db.db.Query(ctx, query, bindVars[0])
	} else {
		cursor, err = db.db.Query(ctx, query, nil)
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

func (db *ArangoDB) Graph(ctx context.Context) (*model.Graph, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := QueryReadAll[Node](ctx, db, `FOR n in nodes RETURN n`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query nodes")
	}

	edges, err := QueryReadAll[Edge](ctx, db, `FOR e in edges RETURN e`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query edges")
	}

	lang := middleware.CtxGetLanguage(ctx)
	return NewConvertToModel(lang).Graph(nodes, edges), nil
}

// XXX: how to test if begin/end transaction lock the right databases?
func (db *ArangoDB) beginTransaction(ctx context.Context, cols driver.TransactionCollections) (driver.TransactionID, error) {
	stamp, ok := ctx.Deadline()
	var opts *driver.BeginTransactionOptions
	if ok {
		opts = &driver.BeginTransactionOptions{LockTimeout: time.Now().Sub(stamp)}
	}
	return db.db.BeginTransaction(ctx, cols, opts)
}

func (db *ArangoDB) endTransaction(ctx context.Context, transaction driver.TransactionID, err *error) {
	if *err != nil {
		*err = db.db.AbortTransaction(ctx, transaction, nil)
	} else {
		*err = db.db.CommitTransaction(ctx, transaction, nil)
	}
}

func (db *ArangoDB) CreateNode(ctx context.Context, user User, description *model.Text) (string, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return "", err
	}
	transaction, err := db.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_NODES, COLLECTION_NODEEDITS}})
	defer db.endTransaction(ctx, transaction, &err)
	col, err := db.db.Collection(ctx, COLLECTION_NODES)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODES)
	}
	node := Node{
		Description: ConvertToDBText(description),
	}
	meta, err := col.CreateDocument(ctx, node)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create node '%v', meta: '%v'", node, meta)
	}
	node.Key = meta.ID.Key()
	col, err = db.db.Collection(ctx, COLLECTION_NODEEDITS)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODEEDITS)
	}
	nodeedit := NodeEdit{
		Node: node.Key,
		User: user.Key,
		Type: NodeEditTypeCreate,
	}
	meta, err = col.CreateDocument(ctx, nodeedit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create nodeedit '%v', meta: '%v'", nodeedit, meta)
	}
	return node.Key, err
}

// TODO(skep): unify collection selection -- should happen outside of DB
// content handling (i.e. inside './internal/controller/'!)
func AddNodePrefix(nodeID string) string {
	return COLLECTION_NODES + "/" + nodeID
}

// CreateEdge creates an edge from node `from` to node `to` with weight `weight`.
// Nodes use ArangoDB format <collection>/<nodeID>.
// Returns the ID of the created edge and nil on success. On failure an empty
// string and an error is returned.
func (db *ArangoDB) CreateEdge(ctx context.Context, user User, from, to string, weight float64) (string, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return "", err
	}
	if from == to {
		return "", errors.Errorf("no self-linking nodes allowed (from == to == '%s')", from)
	}
	transaction, err := db.beginTransaction(ctx, driver.TransactionCollections{Read: []string{COLLECTION_NODES}, Write: []string{COLLECTION_EDGES, COLLECTION_EDGEEDITS}})
	defer db.endTransaction(ctx, transaction, &err)
	col, err := db.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	edges, err := QueryReadAll[Edge](ctx, db, `FOR e in edges FILTER e._from == @from AND e._to == @to RETURN e`, map[string]interface{}{
		"from": from, "to": to,
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to query duplicate edges (%v)", edges)
	}
	if len(edges) > 0 {
		return "", errors.Errorf("edge already exists: %v", edges)
	}
	edge := Edge{
		From:   from,
		To:     to,
		Weight: weight,
	}
	if err := db.nodesExist(ctx, []string{from, to}); err != nil {
		return "", err
	}
	meta, err := col.CreateDocument(ctx, &edge)
	edge.Key = meta.ID.Key()
	if err != nil {
		return "", errors.Wrapf(err, "failed to create edge '%v', meta: '%v'", edge, meta)
	}
	col, err = db.db.Collection(ctx, COLLECTION_EDGEEDITS)
	if err != nil {
		return "", errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGEEDITS)
	}
	edit := &EdgeEdit{
		Edge:   edge.Key,
		User:   user.Key,
		Type:   EdgeEditTypeCreate,
		Weight: weight,
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create EdgeEdit '%#v', meta: '%v'", edit, meta)
	}
	return edge.Key, err
}

// nodesExist returns nil if all nodes exist, otherwise on the first
// non-existing node an error is returned
func (db *ArangoDB) nodesExist(ctx context.Context, nodesWithCollection []string) error {
	for _, node := range nodesWithCollection {
		if err := db.nodeExists(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// nodeExists takes a nodeWithCollection in arangoDB format
// <collection>/<nodeID>, returns an error if the node (or collection) does not
// exist
func (db *ArangoDB) nodeExists(ctx context.Context, nodeWithCollection string) error {
	tmp := strings.Split(nodeWithCollection, "/")
	if len(tmp) != 2 {
		return errors.New("internal error: node format invalid")
	}
	collection_name, node_name := tmp[0], tmp[1]
	collection, err := db.db.Collection(ctx, collection_name)
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

func (db *ArangoDB) EditNode(ctx context.Context, user User, nodeID string, description *model.Text) error {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return err
	}
	transaction, err := db.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_NODES, COLLECTION_NODEEDITS}})
	defer db.endTransaction(ctx, transaction, &err)
	col, err := db.db.Collection(ctx, COLLECTION_NODES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODES)
	}
	node := Node{}
	// Note: currently it is not required to load the old data, since nodes
	// only have the describtion attribute, which is merged on DB level
	meta, err := col.ReadDocument(ctx, nodeID, &node)
	if err != nil {
		return errors.Wrapf(err, "failed to read node id = %s, meta: '%v'", nodeID, meta)
	}
	node.Description = ConvertToDBText(description)
	// merged on db level
	//node.Description = MergeText(node.Description, ConvertToDBText(description))
	meta, err = col.UpdateDocument(ctx, nodeID, &node)
	if err != nil {
		return errors.Wrapf(err, "failed to update node id = %s, node: %v, meta: '%v'", nodeID, node, meta)
	}
	col, err = db.db.Collection(ctx, COLLECTION_NODEEDITS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_NODEEDITS)
	}
	edit := NodeEdit{
		Node: nodeID,
		User: user.Key,
		Type: NodeEditTypeEdit,
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return errors.Wrapf(err, "failed to create NodeEdit{%v}", edit)
	}
	return err
}

func (db *ArangoDB) AddEdgeWeightVote(ctx context.Context, user User, edgeID string, weight float64) error {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return err
	}
	transaction, err := db.beginTransaction(ctx, driver.TransactionCollections{Write: []string{COLLECTION_EDGES, COLLECTION_EDGEEDITS}})
	defer db.endTransaction(ctx, transaction, &err)
	col, err := db.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	edge := Edge{}
	meta, err := col.ReadDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to read edge: %v", meta)
	}
	// TODO(skep): should move aggregation to separate module/application
	edits, err := QueryReadAll[EdgeEdit](ctx, db, `FOR edit in edgeedits FILTER edit.edge == @edge AND edit.weight != 0 RETURN edit`, map[string]interface{}{
		"edge": edgeID,
	})
	sum := Sum(edits, func(edit EdgeEdit) float64 { return edit.Weight })
	averageWeight := (sum + weight) / float64(len(edits)+1)
	edge.Weight = averageWeight
	meta, err = col.UpdateDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to update edge: %v\nedge: %v", meta, edge)
	}
	col, err = db.db.Collection(ctx, COLLECTION_EDGEEDITS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGEEDITS)
	}
	edit := EdgeEdit{
		User:   user.Key,
		Edge:   edge.Key,
		Type:   EdgeEditTypeVote,
		Weight: weight,
	}
	meta, err = col.CreateDocument(ctx, edit)
	if err != nil {
		return errors.Wrapf(err, "failed to create EdgeEdit{%v}: %v", edit, meta)
	}
	return err
}

func NewArangoDB(conf Config) (DB, error) {
	db := ArangoDB{}
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

func GetAuthentication(conf Config) (driver.Authentication, error) {
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

func (db *ArangoDB) Init(conf Config) (DB, error) {
	var err error
	db.conn, err = http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{conf.Host},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to arangodb instance")
	}
	auth, err := GetAuthentication(conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get authentication data")
	}
	db.cli, err = driver.NewClient(driver.ClientConfig{
		Connection:     db.conn,
		Authentication: auth,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arangodb client")
	}
	return db, nil
}

func (db *ArangoDB) OpenDatabase(ctx context.Context) error {
	var err error
	if db.db != nil {
		return nil
	}
	db.db, err = db.cli.Database(ctx, GRAPH_DB_NAME)
	if err != nil {
		return errors.Wrapf(err, "failed to open database '%s'", GRAPH_DB_NAME)
	}
	return nil
}

var AQL_SCHEMA_VALIDATE = `
let schema = SCHEMA_GET(@collection)
RETURN false NOT IN ( for o in @@collection return SCHEMA_VALIDATE(o, schema).valid)
`

func (db *ArangoDB) validateSchemaForCollection(ctx context.Context, collection string, opts *driver.CollectionSchemaOptions) (bool, error) {
	col, err := db.db.Collection(ctx, collection)
	if err != nil {
		return false, errors.Wrapf(err, "failed to access '%s' collection", collection)
	}
	props, err := col.Properties(ctx)
	if err != nil {
		return false, errors.Wrapf(err, "failed to access '%s' collection properties", collection)
	}
	diff := pretty.Compare(props.Schema, opts) // FIXME(skep): see DeepEqual problem in schema.go
	//if reflect.DeepEqual(props.Schema, opts) {
	if diff == "" {
		return false, nil
	}
	//else {
	//	fmt.Printf("diff:\n%s\n", diff)
	//}
	// You wonder why @collection & @@collection? See
	// https://www.arangodb.com/docs/stable/aql/fundamentals-bind-parameters.html#syntax
	valid, err := QueryReadAll[bool](ctx, db, AQL_SCHEMA_VALIDATE, map[string]interface{}{
		"@collection": collection,
		"collection":  collection,
	})
	if err != nil {
		return true, errors.Wrapf(err, "failed to execute AQL: %v", AQL_SCHEMA_VALIDATE)
	}
	if len(valid) != 1 {
		return true, errors.Errorf("unknown AQL return value\ncurrent/old schema:\n%#v\nnew schema:\n%#v", props.Schema, opts)
	}
	if valid[0] == false {
		return true, errors.Errorf("incompatible schemas!\ncurrent/old schema:\n%#v\nnew schema:\n%#v", props.Schema, opts)
	}
	err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: opts})
	if err != nil {
		return true, errors.Wrapf(err, "failed to set schema options (to collection %s): %v", collection, opts)
	}
	return true, nil
}

// returns true, if schema changed, false otherwise
func (db *ArangoDB) ValidateSchema(ctx context.Context) (bool, error) {
	var changed bool
	for _, collection := range CollectionSpecification {
		changedCur, err := db.validateSchemaForCollection(ctx, collection.Name, collection.Options.Schema)
		changed = changed || changedCur
		if err != nil {
			return changed, errors.Wrapf(err, "validate schema for '%s' failed", collection.Name)
		}
	}
	return changed, nil
}

func (db *ArangoDB) CollectionsExist(ctx context.Context) (bool, error) {
	for _, col := range CollectionSpecification {
		exists, err := db.db.CollectionExists(ctx, col.Name)
		if err != nil {
			return exists, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func (db *ArangoDB) CreateDBWithSchema(ctx context.Context) error {
	exists, err := db.cli.DatabaseExists(ctx, GRAPH_DB_NAME)
	if err != nil {
		return errors.Wrapf(err, "failed to check DB existence `%v`: %v", GRAPH_DB_NAME, db)
	}
	var learngraphDB driver.Database
	if !exists {
		learngraphDB, err = db.cli.CreateDatabase(ctx, GRAPH_DB_NAME, nil) //&driver.CreateDatabaseOptions{})
	} else {
		learngraphDB, err = db.cli.Database(ctx, GRAPH_DB_NAME)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to create/open DB `%v`: %v", GRAPH_DB_NAME, db)
	}
	db.db = learngraphDB

	for _, collection := range CollectionSpecification {
		if exists, err = db.db.CollectionExists(ctx, collection.Name); !exists || err != nil {
			col, err := db.db.CreateCollection(ctx, collection.Name, &collection.Options)
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

	if exists, err = db.db.GraphExists(ctx, "graph"); !exists || err != nil {
		_, err = db.db.CreateGraph(ctx, "graph", &driver.CreateGraphOptions{
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

func QueryExists(ctx context.Context, db *ArangoDB, collection, property, value string) (bool, error) {
	existsQuery := fmt.Sprintf(`RETURN LENGTH(for u in %s FILTER u.%s == @%s LIMIT 1 RETURN u) > 0`, collection, property, property)
	cursor, err := db.db.Query(ctx, existsQuery, map[string]interface{}{
		property: value,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query existance with '%s' for %s '%s'", existsQuery, property, value)
	}
	var exists bool
	cursor.ReadDocument(ctx, &exists)
	return exists, nil
}

func (db *ArangoDB) verifyUserInput(ctx context.Context, user User, password string) (*model.CreateUserResult, error) {
	if len(password) < MIN_PASSWORD_LENGTH {
		msg := fmt.Sprintf("Password must be at least length %d, the provided one has only %d characters.", MIN_PASSWORD_LENGTH, len(password))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, nil
	}
	if len(user.Username) < MIN_USERNAME_LENGTH {
		msg := fmt.Sprintf("Username must be at least length %d, the provided one has only %d characters.", MIN_USERNAME_LENGTH, len(user.Username))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, nil
	}
	if _, err := mail.ParseAddress(user.EMail); err != nil {
		msg := fmt.Sprintf("Invalid EMail: '%s'", user.EMail)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, nil
	}
	if userExists, err := QueryExists(ctx, db, COLLECTION_USERS, "username", user.Username); err != nil || userExists {
		msg := fmt.Sprintf("Username already exists: '%s'", user.Username)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, err
	}
	if emailExists, err := QueryExists(ctx, db, COLLECTION_USERS, "email", user.EMail); err != nil || emailExists {
		msg := fmt.Sprintf("EMail already exists: '%s'", user.EMail)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}, err
	}
	return nil, nil
}

func makeNewAuthenticationToken() AuthenticationToken {
	return AuthenticationToken{
		Token:  uuid.New().String(),
		Expiry: time.Now().Add(AUTHENTICATION_TOKEN_EXPIRY).UnixMilli(),
	}
}

func (db *ArangoDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return nil, err
	}
	user := User{
		Username: username,
		EMail:    email,
	}
	invalidInput, err := db.verifyUserInput(ctx, user, password)
	if err != nil {
		return nil, err
	}
	if invalidInput != nil {
		return invalidInput, nil
	}
	return db.createUser(ctx, user, password)
}

func (db *ArangoDB) createUser(ctx context.Context, user User, password string) (*model.CreateUserResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create password hash for user '%v'", user)
	}
	user.PasswordHash = string(hash)
	user.Tokens = []AuthenticationToken{makeNewAuthenticationToken()}
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
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

func (db *ArangoDB) Login(ctx context.Context, auth model.LoginAuthentication) (*model.LoginResult, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return nil, err
	}
	user, err := db.getUserByProperty(ctx, "email", auth.Email)
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

	newToken := makeNewAuthenticationToken()
	user.Tokens = append(user.Tokens, newToken)
	updateQuery := `UPDATE { _key: @userkey, authenticationtokens: @authtokens } IN users`
	_, err = db.db.Query(ctx, updateQuery, map[string]interface{}{
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
func (db *ArangoDB) getUserByProperty(ctx context.Context, property, value string) (*User, error) {
	query := fmt.Sprintf("FOR u in users FILTER u.%s == @%s RETURN u", property, property)
	users, err := QueryReadAll[User](ctx, db, query,
		map[string]interface{}{property: value})
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving user with %s='%s'", property, value)
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (db *ArangoDB) deleteUserByKey(ctx context.Context, key string) error {
	user, err := db.getUserByProperty(ctx, "_key", key)
	if err != nil {
		return errors.Wrapf(err, "failed to get user by _key='%s'", key)
	}
	if user == nil {
		return errors.Errorf("no user with _key='%s' exists", key)
	}
	if FindFirst(user.Tokens, isValidToken(middleware.CtxGetAuthentication(ctx))) == nil {
		return errors.Errorf("not authenticated to delete user key='%s'", key)
	}
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
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
func (db *ArangoDB) DeleteAccount(ctx context.Context) error {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return err
	}
	key := middleware.CtxGetUserID(ctx)
	if key == "" {
		return errors.New("no userID in HTTP-header found")
	}
	return db.deleteUserByKey(ctx, key)
}

func (db *ArangoDB) Logout(ctx context.Context) error {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return err
	}

	key := middleware.CtxGetUserID(ctx)
	if key == "" {
		return errors.New("missing userID in HTTP-header: cannot logout")
	}
	user, err := db.getUserByProperty(ctx, "_key", key)
	if err != nil {
		return errors.Wrapf(err, "failed to query user via userID '%s'", key)
	}
	if user == nil {
		return errors.Errorf("no user found with userID '%s'", key)
	}
	activeTokens := user.Tokens
	token := middleware.CtxGetAuthentication(ctx)
	if !ContainsP(activeTokens, token, accessAuthTokenString) {
		return errors.Errorf("not authenticated to logout user key='%s'", key)
	}
	user.Tokens = RemoveIf(activeTokens, func(t AuthenticationToken) bool { return t.Token == token })
	updateQuery := `UPDATE { _key: @userkey, authenticationtokens: @authtokens } IN users`
	_, err = db.db.Query(ctx, updateQuery, map[string]interface{}{
		"userkey":    user.Key,
		"authtokens": user.Tokens,
	})
	if err != nil {
		return errors.Wrapf(err, "query '%s' failed", updateQuery)
	}
	return nil
}

func accessAuthTokenString(t AuthenticationToken) string { return t.Token }

func (db *ArangoDB) getAuthenticatedUser(ctx context.Context) (*User, error) {
	id, token := middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx)
	user, err := db.getUserByProperty(ctx, "_key", id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user")
	}
	if user == nil {
		return nil, nil // user does not exist
	}
	if FindFirst(user.Tokens, isValidToken(token)) == nil {
		return nil, nil // no matching & valid token
	}
	return user, nil
}

func (db *ArangoDB) IsUserAuthenticated(ctx context.Context) (bool, *User, error) {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return false, nil, err
	}
	user, err := db.getAuthenticatedUser(ctx)
	return user != nil, user, err
}

func isValidToken(token string) func(t AuthenticationToken) bool {
	return func(t AuthenticationToken) bool {
		if t.Token == token && time.UnixMilli(t.Expiry).After(time.Now()) {
			return true
		}
		return false
	}
}

func (db *ArangoDB) DeleteAccountWithData(ctx context.Context, username, adminkey string) error {
	err := EnsureSchema(db, ctx)
	if err != nil {
		return err
	}
	targetUser, err := db.getUserByProperty(ctx, "username", username)
	if err != nil {
		return errors.Wrapf(err, "failed to get user by name '%s'", username)
	}
	if targetUser == nil {
		return errors.Errorf("user with name '%s' does not exists", username)
	}
	currentUser, err := db.getAuthenticatedUser(ctx)
	if err != nil {
		return err
	}
	if currentUser == nil {
		return errors.Errorf("userID='%s' is not authenticated / non-existent", middleware.CtxGetUserID(ctx))
	}
	if !Contains(currentUser.Roles, RoleAdmin) {
		return errors.Errorf("user '%s' has does not have role '%s'", username, RoleAdmin)
	}
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_USERS)
	}
	meta, err := col.RemoveDocument(ctx, targetUser.Key)
	if err != nil {
		return errors.Wrapf(err, "failed to remove user with key='%s', meta=%v", targetUser.Key, meta)
	}
	return nil
}
