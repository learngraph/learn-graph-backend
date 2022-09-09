package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/arangodb/go-driver/jwt"
	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

const (
	GRAPH_DB_NAME       = `learngraph`
	COLLECTION_VERTICES = `vertices`
	COLLECTION_EDGES    = `edges`
)

//go:generate mockgen -destination arangodboperations_mock.go -package db . ArangoDBOperations
type ArangoDBOperations interface {
	Init(conf Config) (DB, error)
	OpenDatabase(ctx context.Context) error
	CreateDBWithSchema(ctx context.Context) error
	ValidateSchema(ctx context.Context) (bool, error)
}

type ArangoDB struct {
	conn driver.Connection
	cli  driver.Client
	db   driver.Database
}

type ArangoDocument struct {
	Key string `json:"_key"`
}

type Vertex struct {
	ArangoDocument
	Description string `json:"description"`
}

type Edge struct {
	ArangoDocument
	From string `json:"_from"`
	To   string `json:"_to"`
	Name string `json:"name"`
}

func QueryReadAll[T any](ctx context.Context, db *ArangoDB, query string, bindVars ...map[string]interface{}) ([]T, error) {
	ctx = driver.WithQueryCount(ctx, true) // needed to call .Count() on the cursor below
	var (
		c   driver.Cursor
		err error
	)
	if len(bindVars) == 1 {
		c, err = db.db.Query(ctx, query, bindVars[0])
	} else {
		c, err = db.db.Query(ctx, query, nil)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "query '%s' failed", query)
	}

	out := make([]T, c.Count())
	for i := int64(0); i < c.Count(); i++ {
		t := new(T)
		meta, err := c.ReadDocument(ctx, t)
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

	vertices, err := QueryReadAll[Vertex](ctx, db, `FOR v in vertices RETURN v`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query vertices")
	}

	edges, err := QueryReadAll[Edge](ctx, db, `FOR e in edges RETURN e`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query edges")
	}

	return ModelFromDB(vertices, edges), nil
}

func ModelFromDB(vertices []Vertex, edges []Edge) *model.Graph {
	g := model.Graph{}
	for _, v := range vertices {
		g.Nodes = append(g.Nodes, &model.Node{
			ID: v.Key,
		})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, &model.Edge{
			ID:   e.Key,
			From: e.From,
			To:   e.To,
		})
	}
	return &g
}

func NewArangoDB(conf Config) (DB, error) {
	db := ArangoDB{}
	return db.Init(conf)
}

func ReadSecretFile(file string) (string, error) {
	tmp, err := ioutil.ReadFile(file)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read JWT secret from file '%s'", file)
	}
	if len(tmp) == 0 {
		return "", fmt.Errorf("JWT secret file '%s' is empty", file)
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
		return errors.Wrap(err, "failed to open database")
	}
	return nil
}

var AQL_SCHEMA_VALIDATE = `
let schema = SCHEMA_GET(@collection)
for o in @@collection
    return {"valid":SCHEMA_VALIDATE(o, schema).valid, "obj":o}
`

func All[T any, A ~[]T](ar A, pred func(T) bool) bool {
	for _, a := range ar {
		if !pred(a) {
			return false
		}
	}
	return true
}

func (db *ArangoDB) validateSchemaForCollection(ctx context.Context, collection string, opts *driver.CollectionSchemaOptions) (bool, error) {
	col, err := db.db.Collection(ctx, collection)
	if err != nil {
		return false, errors.Wrapf(err, "failed to access %s collection", collection)
	}
	props, err := col.Properties(ctx)
	if err != nil {
		return false, errors.Wrapf(err, "failed to access %s collection properties", collection)
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
		return true, fmt.Errorf("incompatible schemas!\ncurrent/old schema:\n%#v\nnew schema:\n%#v", props.Schema, opts)
	}
	err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: opts})
	if err != nil {
		return true, errors.Wrapf(err, "failed to set schema options (to collection %s): %v", collection, opts)
	}
	return true, nil
}

// returns true, if schema changed, false otherwise
func (db *ArangoDB) ValidateSchema(ctx context.Context) (bool, error) {
	return db.validateSchemaForCollection(ctx, COLLECTION_VERTICES, &SchemaOptionsVertex)
	// TODO: validate edges as well
	//changedV, errV := db.validateSchemaForCollection(ctx, COLLECTION_VERTICES, &SchemaOptionsVertex)
	//if errV != nil {
	//	return changedV, errors.Wrap(errV, "validate schema for vertices failed")
	//}
	//changedE, errE := db.validateSchemaForCollection(ctx, COLLECTION_EDGES, &SchemaOptionsEdge)
	//changed := changedV || changedE
	//if errE != nil {
	//	return changed, errors.Wrap(errE, "validate schema for edges failed")
	//}
	//return changed, nil
}

// Note: cannot use []string here, as we must ensure unmarshalling creates the
// same types, same goes for the maps below
var SchemaRequiredPropertiesVertice = []interface{}{"description"}
var SchemaRequiredPropertiesEdge = []interface{}{"weight"}

var SchemaPropertyRulesVertice = map[string]interface{}{
	"properties": map[string]interface{}{
		"description": map[string]interface{}{
			"type": "string",
		},
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesVertice,
}
var SchemaPropertyRulesEdge = map[string]interface{}{
	"properties": map[string]interface{}{
		"weight": map[string]interface{}{
			"type":             "number",
			"exclusiveMinimum": true,
			"minimum":          0,
			"exclusiveMaximum": false,
			"maximum":          10,
		},
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesEdge,
}
var SchemaOptionsVertex = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesVertice,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Required properties: %v", SchemaRequiredPropertiesVertice),
}
var SchemaOptionsEdge = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesEdge,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Required properties: %v", SchemaRequiredPropertiesEdge),
}

func (db *ArangoDB) CreateDBWithSchema(ctx context.Context) error {
	learngraphDB, err := db.cli.CreateDatabase(ctx, GRAPH_DB_NAME, nil) //&driver.CreateDatabaseOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create DB `%v`: %v", GRAPH_DB_NAME, db)
	}
	db.db = learngraphDB

	vertice_opts := driver.CreateCollectionOptions{
		Type:   driver.CollectionTypeDocument,
		Schema: &SchemaOptionsVertex,
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_VERTICES, &vertice_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_VERTICES)
	}

	edge_opts := driver.CreateCollectionOptions{
		Type:   driver.CollectionTypeEdge,
		Schema: &SchemaOptionsEdge,
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_EDGES, &edge_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_EDGES)
	}

	_, err = db.db.CreateGraph(ctx, "graph", &driver.CreateGraphOptions{
		EdgeDefinitions: []driver.EdgeDefinition{
			{
				Collection: COLLECTION_EDGES,
				To:         []string{COLLECTION_VERTICES},
				From:       []string{COLLECTION_VERTICES},
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
			err = db.CreateDBWithSchema(ctx)
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
