package db

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/arangodb/go-driver/jwt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"golang.org/x/crypto/bcrypt"
)

const (
	GRAPH_DB_NAME    = `learngraph`
	COLLECTION_NODES = `nodes`
	COLLECTION_EDGES = `edges`
	COLLECTION_USERS = `users`

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
}

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

// arangoDB edge collection, with custom additional fields
type Edge struct {
	Document
	From   string  `json:"_from"`
	To     string  `json:"_to"`
	Weight float64 `json:"weight"`
}

type User struct {
	Document
	Name         string                `json:"username"`
	PasswordHash string                `json:"passwordhash"`
	EMail        string                `json:"email"`
	Tokens       []AuthenticationToken `json:"authenticationtokens,omitempty"`
}

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

func (db *ArangoDB) CreateNode(ctx context.Context, description *model.Text) (string, error) {
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
	return meta.ID.Key(), nil
}

// TODO(skep): unify collection selection -- should happen outside of DB
// content handling (i.e. not in db/...)
func AddNodePrefix(nodeID string) string {
	return COLLECTION_NODES + "/" + nodeID
}

// CreateEdge creates an edge from node `from` to node `to` with weight `weight`.
// Nodes use ArangoDB format <collection>/<nodeID>.
// Returns the ID of the created edge and nil on success. On failure an empty
// string and an error is returned.
func (db *ArangoDB) CreateEdge(ctx context.Context, from, to string, weight float64) (string, error) {
	if from == to {
		return "", errors.Errorf("no self-linking nodes allowed (from == to == '%s')", from)
	}
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
	if err != nil {
		return "", errors.Wrapf(err, "failed to create edge '%v', meta: '%v'", edge, meta)
	}
	return meta.ID.Key(), nil
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
			return errors.Wrapf(err, "cannot create edge: node existance check failed for '%s': '%v'", node_name, err) // TODO: add err to msg
		}
		return errors.Errorf("cannot create edge: node '%s' does not exist", node_name)
	}
	return nil
}

func (db *ArangoDB) EditNode(ctx context.Context, nodeID string, description *model.Text) error {
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
	return nil
}

func (db *ArangoDB) SetEdgeWeight(ctx context.Context, edgeID string, weight float64) error {
	col, err := db.db.Collection(ctx, COLLECTION_EDGES)
	if err != nil {
		return errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_EDGES)
	}
	edge := Edge{}
	meta, err := col.ReadDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to read edge: %v", meta)
	}
	edge.Weight = weight
	meta, err = col.UpdateDocument(ctx, edgeID, &edge)
	if err != nil {
		return errors.Wrapf(err, "failed to update edge: %v\nedge: %v", meta, edge)
	}
	return nil
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
for o in @@collection
    return {"valid":SCHEMA_VALIDATE(o, schema).valid, "obj":o}
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
	if reflect.DeepEqual(props.Schema, opts) {
		return false, nil
	}
	// You wonder why @collection & @@collection? See
	// https://www.arangodb.com/docs/stable/aql/fundamentals-bind-parameters.html#syntax
	valids, err := QueryReadAll[map[string]interface{}](ctx, db, AQL_SCHEMA_VALIDATE, map[string]interface{}{
		"@collection": collection,
		"collection":  collection,
	})
	if err != nil {
		return true, errors.Wrapf(err, "failed to execute AQL: %v", AQL_SCHEMA_VALIDATE)
	}
	if !All(valids, func(v map[string]interface{}) bool { return v["valid"].(bool) }) {
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
	changedV, errV := db.validateSchemaForCollection(ctx, COLLECTION_NODES, &SchemaOptionsNode)
	if errV != nil {
		return changedV, errors.Wrap(errV, "validate schema for nodes failed")
	}
	changedE, errE := db.validateSchemaForCollection(ctx, COLLECTION_EDGES, &SchemaOptionsEdge)
	changed := changedV || changedE
	if errE != nil {
		return changed, errors.Wrap(errE, "validate schema for edges failed")
	}
	return changed, nil
}

// Note: cannot use []string here, as we must ensure unmarshalling creates the
// same types, same goes for the maps below
var SchemaRequiredPropertiesNodes = []interface{}{"description"}
var SchemaRequiredPropertiesEdge = []interface{}{"weight"}
var SchemaRequiredPropertiesUser = []interface{}{"username"}

var SchemaObjectTextTranslations = map[string]interface{}{
	"type":          "object",
	"minProperties": float64(1),
	"properties": map[string]interface{}{
		"en": map[string]interface{}{"type": "string"},
		"de": map[string]interface{}{"type": "string"},
		"ch": map[string]interface{}{"type": "string"},
	},
	"additionalProperties": false,
}
var SchemaObjectAuthenticationToken = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"token":  map[string]interface{}{"type": "string"},
		"expiry": map[string]interface{}{"type": "number"}, // , "format": "date-time"},
	},
	"required": []interface{}{"token", "expiry"},
}

var SchemaPropertyRulesNode = map[string]interface{}{
	"properties": map[string]interface{}{
		"description": SchemaObjectTextTranslations,
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesNodes,
}
var SchemaPropertyRulesEdge = map[string]interface{}{
	"properties": map[string]interface{}{
		"weight": map[string]interface{}{
			"type":             "number",
			"exclusiveMinimum": true,
			"minimum":          float64(0),
			"exclusiveMaximum": false,
			"maximum":          float64(10),
		},
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesEdge,
}
var SchemaPropertyRulesUser = map[string]interface{}{
	"properties": map[string]interface{}{
		"username":     map[string]interface{}{"type": "string"},
		"email":        map[string]interface{}{"type": "string", "format": "email"},
		"passwordhash": map[string]interface{}{"type": "string"},
		"authenticationtokens": map[string]interface{}{
			"type":  "array",
			"items": SchemaObjectAuthenticationToken,
		},
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesUser,
}
var SchemaOptionsNode = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesNode,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesNode),
}
var SchemaOptionsEdge = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesEdge,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesEdge),
}
var SchemaOptionsUser = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesUser,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesUser),
}

func (db *ArangoDB) CreateDBWithSchema(ctx context.Context) error {
	learngraphDB, err := db.cli.CreateDatabase(ctx, GRAPH_DB_NAME, nil) //&driver.CreateDatabaseOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create DB `%v`: %v", GRAPH_DB_NAME, db)
	}
	db.db = learngraphDB

	node_opts := driver.CreateCollectionOptions{
		Type:   driver.CollectionTypeDocument,
		Schema: &SchemaOptionsNode,
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_NODES, &node_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_NODES)
	}

	edge_opts := driver.CreateCollectionOptions{
		Type:   driver.CollectionTypeEdge,
		Schema: &SchemaOptionsEdge,
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_EDGES, &edge_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_EDGES)
	}

	user_opts := driver.CreateCollectionOptions{
		Type:   driver.CollectionTypeDocument,
		Schema: &SchemaOptionsUser,
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_USERS, &user_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_USERS)
	}

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

	return nil
}

func EnsureSchema(db ArangoDBOperations, ctx context.Context) error {
	err := db.OpenDatabase(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "database not found") {
			db.CreateDBWithSchema(ctx)
			// FIXME: should return ^err
		} else {
			return err
		}
		err = db.OpenDatabase(ctx)
	}
	if err != nil {
		return err
	}
	_, err = db.ValidateSchema(ctx)
	return err
}

func verifyUserInput(username, password, email string) *model.CreateUserResult {
	if len(password) < MIN_PASSWORD_LENGTH {
		msg := fmt.Sprintf("Password must be at least length %d, the provided one has only %d characters.", MIN_PASSWORD_LENGTH, len(password))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if len(username) < MIN_USERNAME_LENGTH {
		msg := fmt.Sprintf("Username must be at least length %d, the provided one has only %d characters.", MIN_USERNAME_LENGTH, len(username))
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		msg := fmt.Sprintf("Invalid EMail: '%s'", email)
		return &model.CreateUserResult{Login: &model.LoginResult{Success: false, Message: &msg}}
	}
	return nil
}

func makeNewAuthenticationToken() AuthenticationToken {
	return AuthenticationToken{
		Token:  uuid.New().String(),
		Expiry: time.Now().Add(AUTHENTICATION_TOKEN_EXPIRY).UnixMilli(),
	}
}

func (db *ArangoDB) CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error) {
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access '%s' collection", COLLECTION_USERS)
	}
	if invalidInput := verifyUserInput(username, password, email); invalidInput != nil {
		return invalidInput, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create password hash for user '%v', email '%v'", username, email)
	}
	user := User{
		Name:         username,
		PasswordHash: string(hash),
		EMail:        email,
		Tokens: []AuthenticationToken{
			makeNewAuthenticationToken(),
		},
	}
	meta, err := col.CreateDocument(ctx, user)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create user '%#v', meta: '%v'", user, meta)
	}
	return &model.CreateUserResult{
		NewUserID: meta.ID.Key(),
		Login: &model.LoginResult{
			Success: true,
			Token:   user.Tokens[0].Token,
		},
	}, nil
}

func (db *ArangoDB) Login(ctx context.Context, email, password string) (*model.LoginResult, error) {
	users, err := QueryReadAll[User](ctx, db, "FOR u in users FILTER u.email == @email RETURN u", map[string]interface{}{"email": email})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query user via email '%s'", email)
	}
	if len(users) == 0 {
		msg := "User does not exist"
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
	}
	user := users[0]
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		msg := "Password missmatch"
		return &model.LoginResult{
			Success: false,
			Message: &msg,
		}, nil
	}

	newToken := makeNewAuthenticationToken()
	user.Tokens = append(user.Tokens, newToken)
	updateQuery := `UPDATE { _key: @UserKey, authenticationtokens: @authtokens } IN users`
	_, err = db.db.Query(ctx, updateQuery, map[string]interface{}{
		"UserKey":    user.Key,
		"authtokens": user.Tokens,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "query '%s' failed", updateQuery)
	}

	return &model.LoginResult{
		Success: true,
		Token:   newToken.Token,
	}, nil
}
