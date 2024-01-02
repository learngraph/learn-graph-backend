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
	pg.db.Exec(`DROP TABLE IF EXISTS edges CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS nodeedits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS nodes CASCADE`)
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
	pg := setupDB(t)
	assert := assert.New(t)
	ctx := context.Background()
	description := model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}}
	id, err := pg.CreateNode(ctx, db.User{}, &description)
	if !assert.NoError(err) {
		return
	}
	assert.NotEmpty(id)
	nodes := []Node{}
	assert.NoError(pg.db.Find(&nodes).Error)
	assert.Len(nodes, 1)
	assert.Equal(db.Text{"en": "ok"}, nodes[0].Description)
	editnodes := []NodeEdit{}
	assert.NoError(pg.db.Find(&editnodes).Error)
	//assert.Len(editnodes, 1) // TODO: continue here
}

func TestPostgresDB_EditNode(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Before   Node
		Add      []*model.Translation
		Expected db.Text
	}{
		{
			Name:     "good case",
			Before:   Node{Description: db.Text{"en": "A"}},
			Add:      []*model.Translation{{Language: "en", Content: "B"}},
			Expected: db.Text{"en": "B"},
		},
		{
			Name:     "merge 2 languages",
			Before:   Node{Description: db.Text{"de": "A"}},
			Add:      []*model.Translation{{Language: "en", Content: "B"}},
			Expected: db.Text{"de": "A", "en": "B"},
		},
		{
			Name:   "merge multiple languages",
			Before: Node{Description: db.Text{"de": "A"}},
			Add: []*model.Translation{
				{Language: "en", Content: "B"},
				{Language: "zh", Content: "C"},
			},
			Expected: db.Text{"de": "A", "en": "B", "zh": "C"},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			tx := pg.db.Create(&test.Before)
			assert.NoError(tx.Error)
			err := pg.EditNode(ctx, db.User{}, itoa(test.Before.ID), &model.Text{Translations: test.Add})
			assert.NoError(err)
			nodes := []Node{}
			assert.NoError(pg.db.Find(&nodes).Error)
			assert.Len(nodes, 1)
			assert.Equal(test.Expected, nodes[0].Description)
		})
	}
}

func TestPostgresDB_CreateEdge(t *testing.T) {
	for _, test := range []struct {
		Name       string
		EdgeExists bool
	}{
		{
			Name: "good case",
		},
		{
			Name:       "edge exists -> err",
			EdgeExists: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			// setup
			A := Node{Description: db.Text{"en": "A"}}
			tx := pg.db.Create(&A)
			assert.NoError(tx.Error)
			B := Node{Description: db.Text{"en": "B"}}
			tx = pg.db.Create(&B)
			assert.NoError(tx.Error)
			if test.EdgeExists {
				tx = pg.db.Create(&Edge{From: A, To: B, Weight: 1.22})
				assert.NoError(tx.Error)
			}
			// call it
			id, err := pg.CreateEdge(ctx, db.User{}, fmt.Sprint(A.ID), fmt.Sprint(B.ID), 3.141)
			if test.EdgeExists {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			assert.NotEmpty(id)
			edges := []Edge{}
			tx = pg.db.Find(&edges)
			assert.NoError(tx.Error)
			assert.Len(edges, 1)
			assert.Equal(3.141, edges[0].Weight)
			assert.Equal(A.ID, edges[0].FromID)
			assert.Equal(B.ID, edges[0].ToID)
		})
	}
}
