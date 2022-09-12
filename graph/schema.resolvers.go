package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"log"

	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

// SubmitVote is the resolver for the submitVote field.
func (r *mutationResolver) SubmitVote(ctx context.Context, source string, target string, value float64) (*model.Error, error) {
	panic(fmt.Errorf("not implemented: SubmitVote - submitVote"))
}

// CreateNode is the resolver for the createNode field.
func (r *mutationResolver) CreateNode(ctx context.Context, description *model.Text) (*model.CreateNodeResult, error) {
	panic(fmt.Errorf("not implemented: CreateNode - createNode"))
}

// EditNode is the resolver for the editNode field.
func (r *mutationResolver) EditNode(ctx context.Context, id string, description *model.Text) (*model.Error, error) {
	panic(fmt.Errorf("not implemented: EditNode - editNode"))
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
