package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

// CreateNode is the resolver for the createNode field.
func (r *mutationResolver) CreateNode(ctx context.Context, description model.Text) (*model.CreateEntityResult, error) {
	return r.Ctrl.CreateNode(ctx, description)
}

// CreateEdge is the resolver for the createEdge field.
func (r *mutationResolver) CreateEdge(ctx context.Context, from string, to string, weight float64) (*model.CreateEntityResult, error) {
	return r.Ctrl.CreateEdge(ctx, from, to, weight)
}

// EditNode is the resolver for the editNode field.
func (r *mutationResolver) EditNode(ctx context.Context, id string, description model.Text) (*model.Status, error) {
	return r.Ctrl.EditNode(ctx, id, description)
}

// SubmitVote is the resolver for the submitVote field.
func (r *mutationResolver) SubmitVote(ctx context.Context, id string, value float64) (*model.Status, error) {
	return r.Ctrl.SubmitVote(ctx, id, value)
}

// DeleteNode is the resolver for the deleteNode field.
func (r *mutationResolver) DeleteNode(ctx context.Context, id string) (*model.Status, error) {
	return r.Ctrl.DeleteNode(ctx, id)
}

// DeleteEdge is the resolver for the deleteEdge field.
func (r *mutationResolver) DeleteEdge(ctx context.Context, id string) (*model.Status, error) {
	return r.Ctrl.DeleteEdge(ctx, id)
}

// CreateUserWithEMail is the resolver for the createUserWithEMail field.
func (r *mutationResolver) CreateUserWithEMail(ctx context.Context, username string, password string, email string) (*model.CreateUserResult, error) {
	result, err := r.Db.CreateUserWithEMail(ctx, username, password, email)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("CreateUserWithEMail() -> %v", result)
	return result, err
}

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, authentication model.LoginAuthentication) (*model.LoginResult, error) {
	res, err := r.Db.Login(ctx, authentication)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("auth=%v, err=%v", authentication, err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("Login() -> %v", res)
	return res, nil
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context) (*model.Status, error) {
	err := r.Db.Logout(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("err=%v", err)
		return nil, err
	}
	return nil, nil
}

// ChangePassword is the resolver for the changePassword field.
func (r *mutationResolver) ChangePassword(ctx context.Context, oldPassword string, newPassword string) (*model.Status, error) {
	log.Ctx(ctx).Error().Msg("Call to not implemented resolver!")
	return nil, errors.New("not implemented: ChangePassword - changePassword")
}

// ResetForgottenPasswordToEMail is the resolver for the resetForgottenPasswordToEMail field.
func (r *mutationResolver) ResetForgottenPasswordToEMail(ctx context.Context, email *string) (*model.Status, error) {
	log.Ctx(ctx).Error().Msg("Call to not implemented resolver!")
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
	return r.Ctrl.Graph(ctx)
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
