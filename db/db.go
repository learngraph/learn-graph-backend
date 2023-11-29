package db

import (
	"context"

	"github.com/caarlos0/env/v6"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type GraphDB interface {
	Graph(ctx context.Context) (*model.Graph, error)
	// returns ID of the created node on success
	CreateNode(ctx context.Context, user User, description *model.Text) (string, error)
	// returns ID of the created edge on success
	CreateEdge(ctx context.Context, user User, from, to string, weight float64) (string, error)
	EditNode(ctx context.Context, user User, nodeID string, description *model.Text) error
	SetEdgeWeight(ctx context.Context, user User, edgeID string, weight float64) error
}

type UserDB interface {
	CreateUserWithEMail(ctx context.Context, username, password, email string) (*model.CreateUserResult, error)
	Login(ctx context.Context, auth model.LoginAuthentication) (*model.LoginResult, error)
	DeleteAccount(ctx context.Context) error
	Logout(ctx context.Context) error
	//ChangePassword(ctx context.Context) error
	IsUserAuthenticated(ctx context.Context) (bool, *User, error)
}

//go:generate mockgen -destination db_mock.go -package db . DB
type DB interface {
	UserDB
	GraphDB
}

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
