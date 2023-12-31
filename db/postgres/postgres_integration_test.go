//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

var testConfig = db.Config{PGHost: "localhost"}

func setupDB(t *testing.T) *PostgresDB {
	assert := assert.New(t)
	pgdb, err := NewPostgresDB(testConfig)
	assert.NoError(err)
	pg := pgdb.(*PostgresDB)
	pg.db.Exec(`DROP TABLE IF EXISTS nodes`)
	pg.db.Exec(`DROP TABLE IF EXISTS edges`)
	pgdb, err = NewPostgresDB(testConfig)
	assert.NoError(err)
	return pgdb.(*PostgresDB)
}

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
	description := model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}}
	id, err := pg.CreateNode(ctx, db.User{}, &description)
	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(id)
}

func TestPostgresDB_CreateEdge(t *testing.T) {
	pg := setupDB(t)
	ctx := context.Background()
	assert := assert.New(t)

	A := Node{Description: db.Text{"en": "A"}}
	tx := pg.db.Create(&A)
	assert.NoError(tx.Error)
	B := Node{Description: db.Text{"en": "B"}}
	tx = pg.db.Create(&B)
	assert.NoError(tx.Error)

	id, err := pg.CreateEdge(ctx, db.User{}, fmt.Sprint(A.ID), fmt.Sprint(B.ID), 3.141)
	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(id)
	edges := []Edge{}
	tx = pg.db.Find(&edges)
	assert.NoError(tx.Error)
	assert.Len(edges, 1)
	// TODO: CONINTUE assert edge content
}
