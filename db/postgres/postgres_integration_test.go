//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"gorm.io/gorm"
)

// config and setup
var testConfig = db.Config{PGHost: "localhost"}
var testTimeNow = time.Date(2000, time.January, 1, 12, 0, 0, 0, time.Local)
var testToken = "123"

func setupDB(t *testing.T) *PostgresDB {
	assert := assert.New(t)
	pgdb, err := NewPostgresDB(testConfig)
	assert.NoError(err)
	pg := pgdb.(*PostgresDB)
	pg.db.Exec(`DROP TABLE IF EXISTS authentication_tokens CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS users CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edge_edits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS edges CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS node_edits CASCADE`)
	pg.db.Exec(`DROP TABLE IF EXISTS nodes CASCADE`)
	pgdb, err = NewPostgresDB(testConfig)
	assert.NoError(err)
	pg = pgdb.(*PostgresDB)
	pg.newToken = func() string { return testToken }
	pg.timeNow = func() time.Time { return testTimeNow }
	return pg
}

func TestPostgresDB_NewPostgresDB(t *testing.T) {
	assert := assert.New(t)
	_, err := NewPostgresDB(testConfig)
	assert.NoError(err)
}

// utility
func strptr(s string) *string {
	return &s
}

func TestPostgresDB_CreateNode(t *testing.T) {
	for _, test := range []struct {
		Name                         string
		Description, Resources       []*model.Translation
		ExpDescription, ExpResources db.Text
	}{
		{
			Name:           "node with description & resources",
			Description:    []*model.Translation{{Language: "en", Content: "A"}},
			Resources:      []*model.Translation{{Language: "en", Content: "B"}},
			ExpDescription: db.Text{"en": "A"},
			ExpResources:   db.Text{"en": "B"},
		},
		{
			Name:           "node with only description",
			Description:    []*model.Translation{{Language: "en", Content: "A"}},
			ExpDescription: db.Text{"en": "A"},
			ExpResources:   db.Text{},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			assert := assert.New(t)
			ctx := context.Background()
			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			assert.NoError(pg.db.Create(&user).Error)
			description := model.Text{Translations: test.Description}
			resources := model.Text{Translations: test.Resources}
			id, err := pg.CreateNode(ctx, db.User{Document: db.Document{Key: itoa(user.ID)}}, &description, &resources)
			if !assert.NoError(err) {
				return
			}
			assert.NotEmpty(id)
			nodes := []Node{}
			assert.NoError(pg.db.Find(&nodes).Error)
			assert.Len(nodes, 1)
			assert.Equal(test.ExpDescription, nodes[0].Description)
			assert.Equal(test.ExpResources, nodes[0].Resources)
			editnodes := []NodeEdit{}
			assert.NoError(pg.db.Find(&editnodes).Error)
			assert.Len(editnodes, 1)
			assert.Equal(db.NodeEditTypeCreate, editnodes[0].Type)
			assert.Equal(test.ExpDescription, editnodes[0].NewDescription)
			assert.Equal(test.ExpResources, editnodes[0].NewResources)
		})
	}
}

func TestPostgresDB_EditNode(t *testing.T) {
	for _, test := range []struct {
		Name           string
		Before         Node
		NewDescription []*model.Translation
		NewResources   []*model.Translation
		ExpDescription db.Text
		ExpResources   db.Text
	}{
		{
			Name:           "single language change",
			Before:         Node{Description: db.Text{"en": "A"}},
			NewDescription: []*model.Translation{{Language: "en", Content: "B"}},
			ExpDescription: db.Text{"en": "B"},
		},
		{
			Name:           "merge 2 languages",
			Before:         Node{Description: db.Text{"de": "A"}},
			NewDescription: []*model.Translation{{Language: "en", Content: "B"}},
			ExpDescription: db.Text{"de": "A", "en": "B"},
		},
		{
			Name:   "merge multiple languages",
			Before: Node{Description: db.Text{"de": "A"}},
			NewDescription: []*model.Translation{
				{Language: "en", Content: "B"},
				{Language: "zh", Content: "C"},
			},
			ExpDescription: db.Text{"de": "A", "en": "B", "zh": "C"},
		},
		{
			Name:           "add resources",
			Before:         Node{Description: db.Text{"en": "A"}},
			NewResources:   []*model.Translation{{Language: "en", Content: "B"}},
			ExpDescription: db.Text{"en": "A"},
			ExpResources:   db.Text{"en": "B"},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			assert.NoError(pg.db.Create(&test.Before).Error)
			user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
			assert.NoError(pg.db.Create(&user).Error)
			err := pg.EditNode(ctx, db.User{Document: db.Document{Key: itoa(user.ID)}}, itoa(test.Before.ID), &model.Text{Translations: test.NewDescription}, &model.Text{Translations: test.NewResources})
			assert.NoError(err)
			nodes := []Node{}
			assert.NoError(pg.db.Find(&nodes).Error)
			assert.Len(nodes, 1)
			assert.Equal(test.ExpDescription, nodes[0].Description)
			assert.Equal(test.ExpResources, nodes[0].Resources)
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
		Name, Username, Password, EMail string
		PreexistingUsers                []User
		ExpError                        bool
	}{
		{
			Name:     "good case",
			Username: "asdf",
			Password: "0123456789",
			EMail:    "me@ok",
		},
		{
			Name:             "username already exists",
			Username:         "asdf",
			Password:         "0123456789",
			EMail:            "me@ok",
			PreexistingUsers: []User{{Username: "asdf", PasswordHash: "000", EMail: "a@b"}},
			ExpError:         true,
		},
		{
			Name:             "email already exists",
			Username:         "asdf",
			Password:         "0123456789",
			EMail:            "a@b",
			PreexistingUsers: []User{{Username: "aaaa", PasswordHash: "000", EMail: "a@b"}},
			ExpError:         true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			res, err := pg.CreateUserWithEMail(ctx, test.Username, test.Password, test.EMail)
			if test.ExpError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			dbuser := User{Username: test.Username}
			assert.NoError(pg.db.Where(&dbuser).Preload("Tokens").First(&dbuser).Error)
			assert.Len(dbuser.Tokens, 1)
			expToken := AuthenticationToken{Token: testToken, Expiry: testTimeNow.Add(AUTHENTICATION_TOKEN_EXPIRY)}
			assert.Equal(expToken.Token, dbuser.Tokens[0].Token)
			assert.Equal(expToken.Expiry, dbuser.Tokens[0].Expiry)
			exp := &model.CreateUserResult{
				Login: &model.LoginResult{Success: true, Token: testToken, UserID: itoa(dbuser.ID), UserName: test.Username},
			}
			assert.Equal(exp, res)
		})
	}
}

func TestPostgresDB_Graph(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Nodes    []Node
		Edges    []Edge
		ExpGraph *model.Graph
	}{
		{
			Name: "2 nodes, 2 edges creating cycle",
			Nodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "A"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "B"}},
			},
			Edges: []Edge{
				{Model: gorm.Model{ID: 3}, FromID: 1, ToID: 2, Weight: 5.0},
				{Model: gorm.Model{ID: 4}, FromID: 2, ToID: 1, Weight: 6.0},
			},
			ExpGraph: &model.Graph{
				Nodes: []*model.Node{
					{ID: "1", Description: "A"},
					{ID: "2", Description: "B"},
				},
				Edges: []*model.Edge{
					{ID: "3", From: "1", To: "2", Weight: 5.0},
					{ID: "4", From: "2", To: "1", Weight: 6.0},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			for _, node := range test.Nodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, edge := range test.Edges {
				assert.NoError(pg.db.Create(&edge).Error)
			}
			graph, err := pg.Graph(ctx)
			assert.NoError(err)
			assert.Equal(test.ExpGraph, graph)
		})
	}
}

func TestPostgresDB_Node(t *testing.T) {
	for _, test := range []struct {
		Name    string
		Nodes   []Node
		ExpNode *model.Node
	}{
		{
			Name: "only description",
			Nodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "A"}},
			},
			ExpNode: &model.Node{ID: "1", Description: "A"},
		},
		{
			Name: "description & resources",
			Nodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "A"}, Resources: db.Text{"en": "B"}},
			},
			ExpNode: &model.Node{ID: "1", Description: "A", Resources: strptr("B")},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			for _, node := range test.Nodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			node, err := pg.Node(ctx, "1")
			assert.NoError(err)
			assert.Equal(test.ExpNode, node)
		})
	}
}

const (
	passwd1234 = "1234567890"
	hash1234   = "$2a$10$H8fNtM7CQpT61P3UVy7mDeAjDDMfXakMVk/CyrNhlUUfGi2iRF9oK"
)

func TestPostgresDB_Login(t *testing.T) {
	for _, test := range []struct {
		Name             string
		Auth             model.LoginAuthentication
		PreexistingUsers []User
		ExpRes           *model.LoginResult
		ExpError         bool
	}{
		{
			Name: "success",
			Auth: model.LoginAuthentication{Email: "a@b", Password: passwd1234},
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: hash1234, EMail: "a@b",
			}},
			ExpRes: &model.LoginResult{
				Success:  true,
				Token:    testToken,
				UserID:   "5",
				UserName: "aaaa",
			},
		},
		{
			Name: "no user with that email",
			Auth: model.LoginAuthentication{Email: "a@b", Password: passwd1234},
			PreexistingUsers: []User{{
				Username: "bbbb", PasswordHash: hash1234, EMail: "c@c",
			}},
			ExpError: true,
		},
		{
			Name: "password hash missmatch",
			Auth: model.LoginAuthentication{Email: "a@b", Password: "iforgotmypassword"},
			PreexistingUsers: []User{{
				Username: "aaaa", PasswordHash: hash1234, EMail: "a@b",
			}},
			ExpRes: &model.LoginResult{
				Success: false,
				Message: strptr("Password missmatch"),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			res, err := pg.Login(ctx, test.Auth)
			if test.ExpError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			assert.Equal(test.ExpRes, res)
		})
	}
}

func TestPostgresDB_IsUserAuthenticated(t *testing.T) {
	for _, test := range []struct {
		Name                            string
		ContextUserID, ContextAuthToken string
		PreexistingUsers                []User
		ExpOK                           bool
		ExpUser                         *db.User
		ExpError                        bool
	}{
		{
			Name:             "auth ok",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: testTimeNow.Add(1 * time.Hour)}},
			}},
			ExpOK:   true,
			ExpUser: &db.User{Document: db.Document{Key: "5"}, Username: "aaaa", EMail: "a@b"},
		},
		{
			Name:             "no matching token found",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "YYY", Expiry: testTimeNow.Add(1 * time.Hour)}},
			}},
			ExpOK: false,
		},
		{
			Name:             "token expired",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: testTimeNow.Add(-1 * time.Hour)}},
			}},
			ExpOK: false,
		},
		{
			Name:             "user not found",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			ExpError:         true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			assert := assert.New(t)
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			ok, user, err := pg.IsUserAuthenticated(ctx)
			if test.ExpError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			assert.Equal(test.ExpOK, ok)
			assert.Equal(test.ExpUser, user)
		})
	}
}

//func TestPostgresDB_(t *testing.T) {
//	for _, test := range []struct {
//		Name       string
//	}{
//		{
//			Name: "what",
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
