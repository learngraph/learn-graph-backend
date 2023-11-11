package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/pkg/errors"
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

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, authentication model.LoginAuthentication) (*model.LoginResult, error) {
	res, err := r.Db.Login(ctx, authentication)
	if err != nil {
		log.Error().Msgf("auth=%v, err=%v", authentication, err)
		return nil, err
	}
	return res, nil
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context) (*model.Status, error) {
	err := r.Db.Logout(ctx)
	if err != nil {
		log.Error().Msgf("err=%v", err)
		return nil, err
	}
	return nil, nil
}

// ChangePassword is the resolver for the changePassword field.
func (r *mutationResolver) ChangePassword(ctx context.Context, oldPassword string, newPassword string) (*model.Status, error) {
	return nil, errors.New("not implemented: ChangePassword - changePassword")
}

// ResetForgottenPasswordToEMail is the resolver for the resetForgottenPasswordToEMail field.
func (r *mutationResolver) ResetForgottenPasswordToEMail(ctx context.Context, email *string) (*model.Status, error) {
	return nil, errors.New("not implemented: ResetForgottenPasswordToEMail - resetForgottenPasswordToEMail")
}

// DeleteAccount is the resolver for the deleteAccount field.
func (r *mutationResolver) DeleteAccount(ctx context.Context) (*model.Status, error) {
	err := r.Db.DeleteAccount(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	return nil, nil
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

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
