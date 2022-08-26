package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

var testConfig = Config{
	User:     "test",
	Password: "test",
	Host:     "http://localhost:18529",
}

func TestNewArangoDB(t *testing.T) {
	_, err := NewArangoDB(testConfig)
	assert.NoError(t, err, "expected connection succeeds")
}

func TestEnsureSchema(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(db *MockArangoDBOperations, ctx context.Context)
		ReturnsError     bool
	}{
		{
			Name: "DB does not exist, creation successful, validation succcessful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does not exist, creation successful, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does not exist, creation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
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

func SetupDB(db *ArangoDB, t *testing.T) {
	err := db.CreateDBWithSchema(context.Background())
	assert.NoError(t, err)
}

func CleanupDB(db *ArangoDB, t *testing.T) {
	if db.db != nil {
		err := db.db.Remove(context.Background())
		assert.NoError(t, err)
	}
	exists, err := db.cli.DatabaseExists(context.Background(), GRAPH_DB_NAME)
	assert.NoError(t, err)
	if !exists {
		return
	}
	thisdb, err := db.cli.Database(context.Background(), GRAPH_DB_NAME)
	assert.NoError(t, err)
	err = thisdb.Remove(context.Background())
	assert.NoError(t, err)
}

func TestArangoDB_CreateDBWithSchema(t *testing.T) {
	db, err := NewArangoDB(testConfig)
	assert.NoError(t, err)
	t.Cleanup(func() { CleanupDB(db.(*ArangoDB), t) })
	SetupDB(db.(*ArangoDB), t)
}

func TestArangoDB_Graph(t *testing.T) {
	db, err := NewArangoDB(testConfig)
	assert.NoError(t, err)
	t.Cleanup(func() { CleanupDB(db.(*ArangoDB), t) })
	d := db.(*ArangoDB)
	SetupDB(d, t)
	ctx := context.Background()
	col, err := d.db.Collection(ctx, COLLECTION_VERTICES)
	assert.NoError(t, err)

	meta, err := col.CreateDocument(ctx, map[string]interface{}{
		"_key": "123",
	})
	assert.NoError(t, err, meta)
	meta, err = col.CreateDocument(ctx, map[string]interface{}{
		"_key": "4",
	})
	assert.NoError(t, err, meta)

	graph, err := db.Graph(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, &model.Graph{
		Nodes: []*model.Node{
			{ID: "123"},
			{ID: "4"},
		},
		Edges: nil,
	}, graph)
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
