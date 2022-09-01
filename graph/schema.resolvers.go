package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"log"

	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

func (r *mutationResolver) CreateTodo(ctx context.Context, input model.NewTodo) (*model.Todo, error) {
	return &model.Todo{
		ID: "12",
	}, nil
}

func (r *queryResolver) Graph(ctx context.Context) (*model.Graph, error) {
	return r.db.Graph(ctx)
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver {
	db, err := db.NewArangoDB(db.GetEnvConfig())
	if err != nil {
		log.Fatal("failed to connect to DB")
	}
	return &queryResolver{Resolver: r, db: db}
}

type mutationResolver struct{ *Resolver }
type queryResolver struct {
	*Resolver
	db db.DB
}
