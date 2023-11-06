package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

// SubmitVote is the resolver for the submitVote field.
func (r *mutationResolver) SubmitVote(ctx context.Context, id string, value float64) (*model.Status, error) {
	err := r.Db.SetEdgeWeight(ctx, id, value)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return nil, nil
}

// CreateNode is the resolver for the createNode field.
func (r *mutationResolver) CreateNode(ctx context.Context, description model.Text) (*model.CreateEntityResult, error) {
	id, err := r.Db.CreateNode(ctx, &description)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return &model.CreateEntityResult{ID: id}, nil
}

// CreateEdge is the resolver for the createEdge field.
func (r *mutationResolver) CreateEdge(ctx context.Context, from string, to string, weight float64) (*model.CreateEntityResult, error) {
	from, to = db.AddNodePrefix(from), db.AddNodePrefix(to)
	ID, err := r.Db.CreateEdge(ctx, from, to, weight)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return &model.CreateEntityResult{ID: ID}, nil
}

// EditNode is the resolver for the editNode field.
func (r *mutationResolver) EditNode(ctx context.Context, id string, description model.Text) (*model.Status, error) {
	err := r.Db.EditNode(ctx, id, &description)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return nil, nil
}

// CreateUserWithEMail is the resolver for the createUserWithEMail field.
func (r *mutationResolver) CreateUserWithEMail(ctx context.Context, user string, password string, email string) (*model.CreateUserResult, error) {
	result, err := r.Db.CreateUserWithEMail(ctx, user, password, email)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return result, err
}

// ChangePassword is the resolver for the changePassword field.
func (r *mutationResolver) ChangePassword(ctx context.Context, user string, oldPassword string, newPassword string) (*model.Status, error) {
	panic(fmt.Errorf("not implemented: ChangePassword - changePassword"))
}

// ResetForgottenPasswordToEMail is the resolver for the resetForgottenPasswordToEMail field.
func (r *mutationResolver) ResetForgottenPasswordToEMail(ctx context.Context, user *string, email *string) (*model.Status, error) {
	panic(fmt.Errorf("not implemented: ResetForgottenPasswordToEMail - resetForgottenPasswordToEMail"))
}

// DeleteAccount is the resolver for the deleteAccount field.
func (r *mutationResolver) DeleteAccount(ctx context.Context, user string) (*model.Status, error) {
	panic(fmt.Errorf("not implemented: DeleteAccount - deleteAccount"))
}

// Graph is the resolver for the graph field.
func (r *queryResolver) Graph(ctx context.Context) (*model.Graph, error) {
	g, err := r.Db.Graph(ctx)
	log := log.Ctx(ctx)
	if err != nil || g == nil {
		log.Error().Msgf("%v | graph=%v", err, g)
	} else if g != nil {
		log.Info().Msgf("returns %d nodes and %d edges", len(g.Nodes), len(g.Edges))
	}
	return g, err
}

// Login is the resolver for the login field.
func (r *queryResolver) Login(ctx context.Context, authentication *model.LoginAuthentication) (*model.LoginResult, error) {
	panic(fmt.Errorf("not implemented: Login - login"))
}

// Logout is the resolver for the logout field.
func (r *queryResolver) Logout(ctx context.Context) (*model.Status, error) {
	panic(fmt.Errorf("not implemented: Logout - logout"))
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
