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
	AddEdgeWeightVote(ctx context.Context, user User, edgeID string, weight float64) error
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
	Node    string       `json:"node"`
	User    string       `json:"user"`
	Type    NodeEditType `json:"type"`
	NewNode Node         `json:"newnode"`
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
