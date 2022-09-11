package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"log"

	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

// CreateTodo is the resolver for the createTodo field.
func (r *mutationResolver) CreateTodo(ctx context.Context, input model.NewTodo) (*model.Todo, error) {
	return &model.Todo{
		ID: "12",
	}, nil
}

// Graph is the resolver for the graph field.
func (r *queryResolver) Graph(ctx context.Context) (*model.Graph, error) {
	g, err := r.Db.Graph(ctx)
	if err != nil || g == nil {
		log.Printf("Graph(): error: %v | graph=%v", err, g)
	} else if g != nil {
		log.Printf("Graph(): returns %d nodes and %d edges", len(g.Nodes), len(g.Edges))
	}
	return g, err
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
