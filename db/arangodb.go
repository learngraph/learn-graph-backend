package db

import (
	"context"
	"os"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
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
	ValidateSchema(ctx context.Context) error
}

type ArangoDB struct {
	conn driver.Connection
	cli  driver.Client
	db   driver.Database
}

//type ArangoDocument struct {
//	Key string `json:"_key"`
//}
type Vertex struct {
	//ArangoDocument
	Key         string `json:"_key"`
	Description string `json:"description"`
}

type Edge struct {
	//ArangoDocument
	Key  string `json:"_key"`
	From string `json:"from"`
	To   string `json:"to"`
	Name string `json:"name"`
}

// TODO: refactor
//func QueryReadAll[T any](ctx context.Context, db *ArangoDB, query string, out []T) error {
//	c, err := db.db.Query(ctx, query, nil)
//	if err != nil {
//		return errors.Wrapf(err, "query '%s' failed", query)
//	}
//	for c.HasMore() {
//		o := T{}
//		meta, err := c.ReadDocument(ctx, &o)
//		if err != nil {
//			return errors.Wrapf(err, "failed to read document: %v", meta)
//		}
//		out = append(out, o)
//	}
//	return nil
//}

func (db *ArangoDB) Graph(ctx context.Context) (*model.Graph, error) {
	c, err := db.db.Query(ctx, `FOR v in vertices RETURN v`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query vertices")
	}
	vertices := []Vertex{}
	for c.HasMore() {
		v := Vertex{}
		meta, err := c.ReadDocument(ctx, &v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read document: %v", meta)
		}
		vertices = append(vertices, v)
	}

	c, err = db.db.Query(ctx, `FOR e in edges RETURN e`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query edges")
	}
	edges := []Edge{}
	for c.HasMore() {
		e := Edge{}
		meta, err := c.ReadDocument(ctx, &e)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read document: %v", meta)
		}
		edges = append(edges, e)
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

func (db *ArangoDB) Init(conf Config) (DB, error) {
	var err error
	db.conn, err = http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{conf.Host},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to arangodb instance")
	}
	db.cli, err = driver.NewClient(driver.ClientConfig{
		Connection:     db.conn,
		Authentication: driver.BasicAuthentication(conf.User, conf.Password),
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

// TODO: use https://www.arangodb.com/docs/stable/aql/functions-miscellaneous.html#schema_validate
const ValidateDBWithSchemaJS = ``

func (db *ArangoDB) ValidateSchema(ctx context.Context) error {
	// TODO: valide all data
	//_, err := db.db.Transaction(ctx, ValidateDBWithSchemaJS, nil)
	//if err != nil {
	//	return errors.Wrap(err, "ValidateSchemaJS failed")
	//}
	return nil
}

func (db *ArangoDB) CreateDBWithSchema(ctx context.Context) error {
	learngraphDB, err := db.cli.CreateDatabase(ctx, GRAPH_DB_NAME, nil) //&driver.CreateDatabaseOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create DB `%v`: %v", GRAPH_DB_NAME, db)
	}
	db.db = learngraphDB

	vertice_opts := driver.CreateCollectionOptions{
		Type: driver.CollectionTypeDocument,
		Schema: &driver.CollectionSchemaOptions{
			Rule: map[string]interface{}{
				"properties": map[string]interface{}{
					"description": map[string]string{
						"type": "string",
					},
				},
				"additionalProperties": false,
				//"required":             []string{},
			},
			Message: "Permitted attributes: { 'description' }",
		},
	}
	_, err = db.db.CreateCollection(ctx, COLLECTION_VERTICES, &vertice_opts)
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' collection", COLLECTION_VERTICES)
	}

	edge_opts := driver.CreateCollectionOptions{
		Type: driver.CollectionTypeEdge,
		Schema: &driver.CollectionSchemaOptions{
			Rule: map[string]interface{}{
				"properties": map[string]interface{}{
					"name": map[string]string{
						"type": "string",
					},
				},
				"additionalProperties": false,
				//"required":             []string{},
			},
			Message: "Permitted attributes: { 'name' }",
		},
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
		if os.IsNotExist(err) {
			err = db.CreateDBWithSchema(ctx)
		} else {
			return err
		}
		err = db.OpenDatabase(ctx)
	}
	if err != nil {
		return err
	}
	return db.ValidateSchema(ctx)
}
