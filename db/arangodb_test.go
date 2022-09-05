package db

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

func TestEnsureSchema(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(db *MockArangoDBOperations, ctx context.Context)
		ReturnsError     bool
	}{
		{
			Name: "DB does not exist, creation successful, validation succcessful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does not exist, creation successful, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does not exist, creation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does exist, validation successful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does exist, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
	} {
		ctrl := gomock.NewController(t)
		t.Log("Running:", test.Name)
		ctx := context.Background()
		db := NewMockArangoDBOperations(ctrl)
		test.MockExpectations(db, ctx)
		err := EnsureSchema(db, ctx)
		if test.ReturnsError {
			assert.Error(t, err, test.Name)
		} else {
			assert.NoError(t, err, test.Name)
		}
	}
}

func TestModelFromDB(t *testing.T) {
	for _, test := range []struct {
		Name string
		Exp  *model.Graph
		InpV []Vertex
		InpE []Edge
	}{
		{
			Name: "single vertex",
			InpV: []Vertex{{ArangoDocument: ArangoDocument{Key: "abc"}}},
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc"},
				},
			},
		},
		{
			Name: "multiple vertices",
			InpV: []Vertex{
				{ArangoDocument: ArangoDocument{Key: "abc"}},
				{ArangoDocument: ArangoDocument{Key: "def"}},
			},
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc"},
					{ID: "def"},
				},
			},
		},
		{
			Name: "2 vertices 1 edge",
			InpV: []Vertex{
				{ArangoDocument: ArangoDocument{Key: "a"}},
				{ArangoDocument: ArangoDocument{Key: "b"}},
			},
			InpE: []Edge{
				{ArangoDocument: ArangoDocument{Key: "?"}, From: "a", To: "b"},
			},
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "a"},
					{ID: "b"},
				},
				Edges: []*model.Edge{
					{ID: "?", From: "a", To: "b"},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, ModelFromDB(test.InpV, test.InpE))
		})
	}
}

func TestGetAuthentication(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Config   Config
		ExpValue string
		ExpError bool
	}{
		{
			Name: "pre-existing token",
			Config: Config{
				JwtToken: "abc",
			},
			ExpValue: "bearer abc",
		},
		{
			Name: "given secret, token must be created",
			Config: Config{
				JwtSecretPath: "./testdata/jwtSecret",
			},
			ExpValue: "bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcmFuZ29kYiIsInNlcnZlcl9pZCI6ImxlYXJuZ3JhcGgtYmFja2VuZCJ9.kKx5li7sgQyBeV1rHix8ngv9XKdoQihXzUnSKxtYp8c",
		},
		{
			Name: "no such file at JwtSecretPath",
			Config: Config{
				JwtSecretPath: "./testdata/doesnotexist",
			},
			ExpError: true,
		},
		{
			Name: "empty file at JwtSecretPath",
			Config: Config{
				JwtSecretPath: "./testdata/emptyfile",
			},
			ExpError: true,
		},
		{
			Name: "skip authentication",
			Config: Config{
				NoAuthentication: true,
			},
			ExpError: false,
			ExpValue: "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			auth, err := GetAuthentication(test.Config)
			if test.ExpError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			if !assert.NotNil(auth) {
				return
			}
			assert.Equal(driver.AuthenticationTypeRaw, auth.Type())
			assert.Equal(test.ExpValue, auth.Get("value"))
		})
	}
}
