package db

import (
	"context"

	"github.com/caarlos0/env/v6"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type DB interface {
	Graph(ctx context.Context) (*model.Graph, error)
	// returns ID of the created node on success
	CreateNode(ctx context.Context, description *model.Text) (string, error)
	// returns ID of the created edge on success
	CreateEdge(ctx context.Context, from, to string) (string, error)
	EditNode(ctx context.Context, nodeID string, description *model.Text) error
	SetEdgeWeight(ctx context.Context, edgeID string, weight float64) error
}

// maybe:
//type ID string

type Config struct {
	Host             string `env:"DB_ARANGO_HOST" envDefault:"http://localhost:8529"`
	JwtToken         string `env:"DB_ARANGO_JWT_TOKEN" envDefault:""`
	JwtSecretPath    string `env:"DB_ARANGO_JWT_SECRET_PATH" envDefault:""`
	NoAuthentication bool   `env:"DB_ARANGO_NO_AUTH" envDefault:"false"`
}

func GetEnvConfig() Config {
	conf := Config{}
	env.Parse(&conf)
	return conf
}
