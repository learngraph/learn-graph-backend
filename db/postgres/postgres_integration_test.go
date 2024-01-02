//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

var testConfig = db.Config{PGHost: "localhost"}

func TestPostgresDB_NewPostgresDB(t *testing.T) {
	assert := assert.New(t)
	_, err := NewPostgresDB(testConfig)
	assert.NoError(err)
}

func TestPostgresDB_CreateNode(t *testing.T) {
	pg, err := NewPostgresDB(testConfig)
	assert := assert.New(t)
	if !assert.NoError(err) {
		return
	}
	ctx := context.Background()
	user := db.User{}
	description := model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}}
	id, err := pg.CreateNode(ctx, user, &description)
	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(id)
}
