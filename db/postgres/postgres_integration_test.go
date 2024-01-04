//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"gorm.io/gorm"
)

var testConfig = db.Config{PGHost: "localhost"}

func setupDB(t *testing.T) *PostgresDB {
	assert := assert.New(t)
	pgdb, err := NewPostgresDB(testConfig)
	assert.NoError(err)
	pg := pgdb.(*PostgresDB)
	pg.db.Exec(`DROP TABLE IF EXISTS users CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edge_edits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edges CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS node_edits CASCADE`)
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
	user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
	assert.NoError(pg.db.Create(&user).Error)
	description := model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}}
	id, err := pg.CreateNode(ctx, db.User{Document: db.Document{Key: itoa(user.ID)}}, &description)
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
	assert.Len(editnodes, 1)
	assert.Equal(db.NodeEditTypeCreate, editnodes[0].Type)
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
			assert.NoError(pg.db.Create(&test.Before).Error)
			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			assert.NoError(pg.db.Create(&user).Error)
			err := pg.EditNode(ctx, db.User{Document: db.Document{Key: itoa(user.ID)}}, itoa(test.Before.ID), &model.Text{Translations: test.Add})
			assert.NoError(err)
			nodes := []Node{}
			assert.NoError(pg.db.Find(&nodes).Error)
			assert.Len(nodes, 1)
			assert.Equal(test.Expected, nodes[0].Description)
			editnodes := []NodeEdit{}
			assert.NoError(pg.db.Find(&editnodes).Error)
			assert.Len(editnodes, 1)
			assert.Equal(db.NodeEditTypeEdit, editnodes[0].Type)
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
			assert.NoError(pg.db.Create(&A).Error)
			B := Node{Description: db.Text{"en": "B"}}
			assert.NoError(pg.db.Create(&B).Error)
			if test.EdgeExists {
				assert.NoError(pg.db.Create(&Edge{From: A, To: B, Weight: 1.22}).Error)
			}
			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			assert.NoError(pg.db.Create(&user).Error)
			// call it
			id, err := pg.CreateEdge(ctx, db.User{Document: db.Document{Key: itoa(user.ID)}}, fmt.Sprint(A.ID), fmt.Sprint(B.ID), 3.141)
			if test.EdgeExists {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			assert.NotEmpty(id)
			edges := []Edge{}
			assert.NoError(pg.db.Find(&edges).Error)
			assert.Len(edges, 1)
			assert.Equal(3.141, edges[0].Weight)
			assert.Equal(A.ID, edges[0].FromID)
			assert.Equal(B.ID, edges[0].ToID)
			edgeedits := []EdgeEdit{}
			assert.NoError(pg.db.Find(&edgeedits).Error)
			assert.Len(edgeedits, 1)
		})
	}
}

func TestPostgresDB_AddEdgeWeightVote(t *testing.T) {
	for _, test := range []struct {
		Name string
	}{
		{
			Name: "good case",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			A := Node{Description: db.Text{"en": "A"}}
			assert.NoError(pg.db.Create(&A).Error)
			B := Node{Description: db.Text{"en": "B"}}
			assert.NoError(pg.db.Create(&B).Error)
			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			assert.NoError(pg.db.Create(&user).Error)
			edge := Edge{Model: gorm.Model{ID: 88}, From: A, To: B, Weight: 10}
			assert.NoError(pg.db.Create(&edge).Error)
			assert.NoError(pg.db.Create(&Edge{Model: gorm.Model{ID: 99}, From: B, To: A, Weight: 5}).Error)
			existing_edits := []EdgeEdit{
				{EdgeID: edge.ID, UserID: user.ID, Weight: 10, Type: db.EdgeEditTypeCreate},
				{EdgeID: 99, UserID: user.ID, Weight: 5, Type: db.EdgeEditTypeCreate},
			}
			assert.NoError(pg.db.Create(&existing_edits).Error)
			arangoUser := db.User{Document: db.Document{Key: itoa(user.ID)}}
			err := pg.AddEdgeWeightVote(ctx, arangoUser, itoa(edge.ID), 4)
			assert.NoError(err)
			edgeedits := []EdgeEdit{}
			assert.NoError(pg.db.Where(&EdgeEdit{EdgeID: edge.ID}).Find(&edgeedits).Error)
			assert.Len(edgeedits, 2)
			assert.NoError(pg.db.First(&edge).Error)
			assert.Equal(7.0, edge.Weight)
		})
	}
}

func TestPostgresDB_CreateUserWithEMail(t *testing.T) {
	for _, test := range []struct {
		Name string
	}{
		{
			Name: "good case",
		},
		// TODO: all them requirements..
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			//user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			//assert.NoError(pg.db.Create(&user).Error)
			res, err := pg.CreateUserWithEMail(ctx, "asdf", "0123456789", "me@ok")
			assert.NoError(err)
			dbuser := User{Username: "asdf"}
			assert.NoError(pg.db.Where(&dbuser).First(&dbuser).Error)
			assert.Equal(&model.CreateUserResult{
				Login: &model.LoginResult{Success: true, Token: "123", UserID: itoa(dbuser.ID), UserName: "asdf"},
			}, res)
		})
	}
}

//func TestPostgresDB_(t *testing.T) {
//	for _, test := range []struct {
//		Name       string
//	}{
//		{
//			Name: "good case",
//		},
//	} {
//		t.Run(test.Name, func(t *testing.T) {
//			pg := setupDB(t)
//			ctx := context.Background()
//			assert := assert.New(t)
//			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
//			assert.NoError(pg.db.Create(&user).Error)
//			arangoUser := db.User{Document: db.Document{Key: itoa(user.ID)}}
//			err := pg.?(ctx, arangoUser, ?)
//		})
//	}
//}
