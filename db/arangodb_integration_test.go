//go:build integration

package db

import (
	"context"
	"testing"

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

func SetupDB(db *ArangoDB, t *testing.T) {
	db.CreateDBWithSchema(context.Background())
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
