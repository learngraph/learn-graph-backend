package db

import (
	"context"
	"os"

	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
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

func (db ArangoDB) Graph(ctx context.Context) (*model.Graph, error) {
	return nil, errors.New("not implemented")
}

func NewArangoDB(conf Config) (DB, error) {
	db := ArangoDB{}
	return db.Init(conf)
}

func (db ArangoDB) Init(conf Config) (DB, error) {
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
	return &db, nil
}

func (db ArangoDB) OpenDatabase(ctx context.Context) error {
	var err error
	if db.db != nil {
		return nil
	}
	db.db, err = db.cli.Database(ctx, "graph")
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	return nil
}

// TODO: use https://www.arangodb.com/docs/stable/aql/functions-miscellaneous.html#schema_validate
const ValidateDBWithSchemaJS = `
`

const CreateDBWithSchemaJS = `
var vertices = {
  rule: { 
    properties: { name: { type: "string" }, 
    additionalProperties: false,
    required: ["name"]
  },
  message: "The document does not contain a string in attribute 'name'."
};
var edges = {
  rule: { 
    properties: { description: { type: "string" }, 
    additionalProperties: false,
    required: []
  },
  message: "The document does not contain a string in attribute 'description'."
};

db._create("vertices", { "schema": vertices });
db._create("edges", { "schema": edges });
`

func (db ArangoDB) ValidateSchema() error {
	//_, err := db.db.Transaction(ctx, ValidateDBWithSchemaJS, nil)
	//if err != nil {
	//	return errors.Wrap(err, "ValidateSchemaJS failed")
	//}
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
