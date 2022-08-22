package db

import (
	"context"

	"github.com/caarlos0/env/v6"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type DB interface {
	Graph(ctx context.Context) (*model.Graph, error)
}

type Config struct {
	Host     string `env:"DB_ARANGO_HOST"`
	User     string `env:"DB_ARANGO_USER"`
	Password string `env:"DB_ARANGO_PASSWORD"`
}

func GetEnvConfig() Config {
	conf := Config{}
	env.Parse(&conf)
	return conf
}
