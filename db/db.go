package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"

	"github.com/caarlos0/env/v6"
	"github.com/pkg/errors"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type GraphDB interface {
	Graph(ctx context.Context) (*model.Graph, error)
	Node(ctx context.Context, ID string) (*model.Node, error)
	// returns ID of the created node on success
	CreateNode(ctx context.Context, user User, description *model.Text, resources *model.Text) (string, error)
	// returns ID of the created edge on success
	CreateEdge(ctx context.Context, user User, from, to string, weight float64) (string, error)
	EditNode(ctx context.Context, user User, nodeID string, description *model.Text, resources *model.Text) error
	AddEdgeWeightVote(ctx context.Context, user User, edgeID string, weight float64) error
	DeleteNode(ctx context.Context, user User, ID string) error
	DeleteEdge(ctx context.Context, user User, ID string) error
	NodeEdits(ctx context.Context, ID string) ([]*model.NodeEdit, error)
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
	PGHost     string `env:"DB_POSTGRES_HOST" envDefault:"localhost"`
	PGPassword string `env:"DB_POSTGRES_PASSWORD" envDefault:"example"`
}

func GetEnvConfig() Config {
	conf := Config{}
	env.Parse(&conf)
	return conf
}

// TODO(skep): remove, was used for arangodb compatibility (which is no longer in use)
type Document struct {
	Key string `json:"_key,omitempty"`
}

type Node struct {
	Document
	Description Text `json:"description"`
	Resources   Text `json:"resources,omitempty"`
}

type NodeEdit struct {
	Document
	Node      string       `json:"node"`
	User      string       `json:"user"`
	Type      NodeEditType `json:"type"`
	NewNode   Node         `json:"newnode"`
	CreatedAt int64        `json:"created_at"`
}

type NodeEditType string

const (
	NodeEditTypeCreate NodeEditType = "create"
	NodeEditTypeEdit   NodeEditType = "edit"
)

type EdgeEdit struct {
	Document
	Edge      string       `json:"edge"`
	User      string       `json:"user"`
	Type      EdgeEditType `json:"type"`
	Weight    float64      `json:"weight"`
	CreatedAt int64        `json:"created_at"`
}

type EdgeEditType string

const (
	EdgeEditTypeCreate EdgeEditType = "create"
	EdgeEditTypeVote   EdgeEditType = "edit"
)

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
	Expiry int64 `json:"expiry"`
}

type Text map[string]string

func (j Text) Value() (driver.Value, error) {
	return json.Marshal(j)
}
func (j *Text) Scan(value interface{}) error {
	if data, ok := value.([]byte); ok {
		return json.Unmarshal(data, &j)
	}
	return errors.Errorf("Failed to unmarshal JSONB value: %v", value)
}

// temporary object for DB migration
type AllData struct {
	Users     []User
	Nodes     []Node
	Edges     []Edge
	NodeEdits []NodeEdit
	EdgeEdits []EdgeEdit
}
