package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"log"

	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

// SubmitVote is the resolver for the submitVote field.
func (r *mutationResolver) SubmitVote(ctx context.Context, id string, value float64) (*model.Status, error) {
	err := r.Db.SetEdgeWeight(ctx, id, value)
	if err != nil {
		log.Printf("error: %v", err)
	}
	return nil, err
}

// CreateNode is the resolver for the createNode field.
func (r *mutationResolver) CreateNode(ctx context.Context, description *model.Text) (*model.CreateEntityResult, error) {
	id, err := r.Db.CreateNode(ctx, description)
	if err != nil {
		return nil, err
	}
	return &model.CreateEntityResult{ID: id}, nil
}

// CreateEdge is the resolver for the createEdge field.
func (r *mutationResolver) CreateEdge(ctx context.Context, from string, to string, weight float64) (*model.CreateEntityResult, error) {
	ID, err := r.Db.CreateEdge(ctx, from, to, weight)
	if err != nil {
		return nil, err
	}
	return &model.CreateEntityResult{ID: ID}, nil
}

// EditNode is the resolver for the editNode field.
func (r *mutationResolver) EditNode(ctx context.Context, id string, description *model.Text) (*model.Status, error) {
	return nil, r.Db.EditNode(ctx, id, description)
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
