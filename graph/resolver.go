package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.
import (
	"github.com/suxatcode/learn-graph-poc-backend/db"
)

type Resolver struct {
	Db db.DB
}
