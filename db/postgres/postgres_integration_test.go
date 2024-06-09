//go:build integration

package postgres

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
	"gorm.io/gorm"
)

// config and setup
func setupDB(t *testing.T) *PostgresDB {
	return TESTONLY_SetupAndCleanup(t)
}

func TestPostgresDB_NewPostgresDB(t *testing.T) {
	assert := assert.New(t)
	_, err := NewPostgresDB(TESTONLY_Config)
	assert.NoError(err)
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
		Name                 string
		TargetEdgeID         uint
		PreexistingNodes     []Node
		PreexistingEdges     []Edge
		PreexistingNodeEdits []NodeEdit
		PreexistingEdgeEdits []EdgeEdit
		PreexistingUsers     []User
		ExpectedWeight       float64
		ExpectedEdgeEdits    int
	}{
		{
			Name:         "two votes from different users",
			TargetEdgeID: 88,
			PreexistingUsers: []User{
				{Model: gorm.Model{ID: 111}, Username: "asdf", PasswordHash: "000", EMail: "a@b"},
				{Model: gorm.Model{ID: 222}, Username: "fasd", PasswordHash: "111", EMail: "c@d"},
			},
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "A"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "B"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 88}, FromID: 1, ToID: 2, Weight: 10},
				{Model: gorm.Model{ID: 99}, FromID: 2, ToID: 1, Weight: 5},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 88, UserID: 111, Weight: 10, Type: db.EdgeEditTypeCreate},
				{EdgeID: 88, UserID: 222, Weight: 5, Type: db.EdgeEditTypeVote},
			},
			ExpectedWeight:    6.33333333333333333,
			ExpectedEdgeEdits: 2,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			for _, node := range test.PreexistingNodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, edge := range test.PreexistingEdges {
				assert.NoError(pg.db.Create(&edge).Error)
			}
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			for _, edgeedit := range test.PreexistingEdgeEdits {
				assert.NoError(pg.db.Create(&edgeedit).Error)
			}
			currentUser := db.User{Document: db.Document{Key: itoa(111)}}
			err := pg.AddEdgeWeightVote(ctx, currentUser, itoa(test.TargetEdgeID), 4)
			assert.NoError(err)
			edgeedits := []EdgeEdit{}
			assert.NoError(pg.db.Where(&EdgeEdit{EdgeID: test.TargetEdgeID}).Find(&edgeedits).Error)
			assert.Len(edgeedits, test.ExpectedEdgeEdits)
			edge := Edge{}
			assert.NoError(pg.db.First(&edge).Error)
			assert.Equal(test.ExpectedWeight, edge.Weight)
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
			expToken := AuthenticationToken{Token: TEST_RandomToken, Expiry: TEST_TimeNow.Add(AUTHENTICATION_TOKEN_EXPIRY)}
			assert.Equal(expToken.Token, dbuser.Tokens[0].Token)
			assert.Equal(expToken.Expiry, dbuser.Tokens[0].Expiry)
			exp := &model.CreateUserResult{
				Login: &model.LoginResult{Success: true, Token: TEST_RandomToken, UserID: itoa(dbuser.ID), UserName: test.Username},
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
			ctx := middleware.TestingCtxNewWithLanguage(context.Background(), "en")
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
			ctx := middleware.TestingCtxNewWithLanguage(context.Background(), "en")
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
				Token:    TEST_RandomToken,
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
			ExpRes: &model.LoginResult{
				Success: false,
				Message: strptr("failed to get user: record not found"),
			},
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
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: TEST_TimeNow.Add(1 * time.Hour)}},
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
				Tokens: []AuthenticationToken{{Token: "YYY", Expiry: TEST_TimeNow.Add(1 * time.Hour)}},
			}},
		},
		{
			Name:             "token expired",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: TEST_TimeNow.Add(-1 * time.Hour)}},
			}},
		},
		{
			Name:             "user not found",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
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

func TestPostgresDB_DeleteNode(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		ExpError             bool
		ExpEdges             []Edge
		NodeIDToDelete       string
		UserID               string
		PreexistingNodes     []Node
		PreexistingNodeEdits []NodeEdit
		PreexistingEdges     []Edge
		PreexistingEdgeEdits []EdgeEdit
		ExpLenNodeEdits      int
	}{
		{
			Name:           "sucess: no edges, no edits",
			NodeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate},
				{NodeID: 2, UserID: 1, Type: db.NodeEditTypeCreate},
				{NodeID: 2, UserID: 2, Type: db.NodeEditTypeEdit},
			},
			ExpLenNodeEdits: 2,
		},
		{
			Name:           "fail: edits present",
			NodeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate},
				{NodeID: 1, UserID: 2 /*other user!*/, Type: db.NodeEditTypeEdit},
			},
			ExpError: true,
		},
		{
			Name:           "fail: edges present from other users",
			NodeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 2, ToID: 1},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 2, Type: db.EdgeEditTypeCreate},
			},
			ExpError: true,
		},
		{
			Name:           "success: edges present but all from this user, delete them all!",
			NodeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
				{Model: gorm.Model{ID: 3}, Description: db.Text{"en": "unrelated c"}},
				{Model: gorm.Model{ID: 4}, Description: db.Text{"en": "unrelated d"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 2, ToID: 1},
				{Model: gorm.Model{ID: 2}, FromID: 3, ToID: 4},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate},
				{EdgeID: 2, UserID: 3, Type: db.EdgeEditTypeCreate},
			},
			ExpEdges: []Edge{
				{Model: gorm.Model{ID: 2}, FromID: 3, ToID: 4},
			},
			ExpError: false,
		},
		{
			Name:           "success: edits present, but admin-role overrides it",
			NodeIDToDelete: "1",
			UserID:         "3", // user with ID 3 is an admin!
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate},
				{NodeID: 1, UserID: 2 /*other user!*/, Type: db.NodeEditTypeEdit},
				{NodeID: 2, UserID: 1, Type: db.NodeEditTypeCreate},
			},
			ExpLenNodeEdits: 1,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			users := []User{
				{Model: gorm.Model{ID: 1}, Username: "current", PasswordHash: "0", EMail: "a@b"},
				{Model: gorm.Model{ID: 2}, Username: "another", PasswordHash: "1", EMail: "c@d"},
				{Model: gorm.Model{ID: 3}, Username: "i'm admin", PasswordHash: "2", EMail: "ad@m",
					Roles: []Role{{UserID: 1, Role: db.RoleAdmin}}},
			}
			for _, user := range users {
				assert.NoError(pg.db.Create(&user).Error)
			}
			for _, node := range test.PreexistingNodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, nodeedit := range test.PreexistingNodeEdits {
				assert.NoError(pg.db.Create(&nodeedit).Error)
			}
			for _, edge := range test.PreexistingEdges {
				assert.NoError(pg.db.Create(&edge).Error)
			}
			for _, edgeedit := range test.PreexistingEdgeEdits {
				assert.NoError(pg.db.Create(&edgeedit).Error)
			}
			currentUser := db.User{Document: db.Document{Key: test.UserID}}
			err := pg.DeleteNode(ctx, currentUser, test.NodeIDToDelete)
			nodeedits := []NodeEdit{}
			assert.NoError(pg.db.Find(&nodeedits).Error)
			if test.ExpError {
				assert.Error(err)
				assert.Len(nodeedits, len(test.PreexistingNodeEdits))
			} else {
				assert.NoError(err)
				assert.Len(nodeedits, test.ExpLenNodeEdits)
			}
			if test.ExpEdges != nil {
				edges := []Edge{}
				assert.NoError(pg.db.Find(&edges).Error)
				assert.Len(edges, len(test.ExpEdges))
			}
		})
	}
}

func TestPostgresDB_DeleteEdge(t *testing.T) {
	for _, test := range []struct {
		Name, EdgeIDToDelete, UserID string
		ExpError                     bool
		PreexistingNodes             []Node
		PreexistingEdges             []Edge
		PreexistingEdgeEdits         []EdgeEdit
		ExpLenEdgeEdits              int
		ExpLenEdges                  int
	}{
		{
			Name:           "success: no edits",
			EdgeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
				{Model: gorm.Model{ID: 3}, Description: db.Text{"en": "c"}},
				{Model: gorm.Model{ID: 4}, Description: db.Text{"en": "d"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 2, ToID: 1},
				{Model: gorm.Model{ID: 2}, FromID: 4, ToID: 3},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate, Weight: 2.2},
				{EdgeID: 2, UserID: 1, Type: db.EdgeEditTypeCreate, Weight: 2.2},
				{EdgeID: 2, UserID: 2, Type: db.EdgeEditTypeVote, Weight: 3.3},
			},
			ExpLenEdges:     1,
			ExpLenEdgeEdits: 2,
		},
		{
			Name:           "fail: votes exist from other users",
			EdgeIDToDelete: "1",
			UserID:         "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 2, ToID: 1},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate},
				{EdgeID: 1, UserID: 2, Type: db.EdgeEditTypeVote},
			},
			ExpError: true,
		},
		{
			Name:           "success: votes exist from other users, but admin-role overrides it",
			EdgeIDToDelete: "1",
			UserID:         "3", // the admin user
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
				{Model: gorm.Model{ID: 3}, Description: db.Text{"en": "c"}},
				{Model: gorm.Model{ID: 4}, Description: db.Text{"en": "d"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 2, ToID: 1},
				{Model: gorm.Model{ID: 2}, FromID: 4, ToID: 3},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate},
				{EdgeID: 1, UserID: 2, Type: db.EdgeEditTypeVote},
				{EdgeID: 2, UserID: 1, Type: db.EdgeEditTypeCreate, Weight: 2.2},
				{EdgeID: 2, UserID: 2, Type: db.EdgeEditTypeVote, Weight: 3.3},
			},
			ExpLenEdges:     1,
			ExpLenEdgeEdits: 2,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			users := []User{
				{Model: gorm.Model{ID: 1}, Username: "current", PasswordHash: "0", EMail: "a@b"},
				{Model: gorm.Model{ID: 2}, Username: "another", PasswordHash: "1", EMail: "c@d"},
				{Model: gorm.Model{ID: 3}, Username: "i'm admin", PasswordHash: "2", EMail: "ad@m",
					Roles: []Role{{Role: db.RoleAdmin}}},
			}
			for _, user := range users {
				assert.NoError(pg.db.Create(&user).Error)
			}
			for _, node := range test.PreexistingNodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, edge := range test.PreexistingEdges {
				assert.NoError(pg.db.Create(&edge).Error)
			}
			for _, edgeedit := range test.PreexistingEdgeEdits {
				assert.NoError(pg.db.Create(&edgeedit).Error)
			}
			currentUser := db.User{Document: db.Document{Key: test.UserID}}
			err := pg.DeleteEdge(ctx, currentUser, test.EdgeIDToDelete)
			edgeedits := []EdgeEdit{}
			assert.NoError(pg.db.Find(&edgeedits).Error)
			edges := []Edge{}
			assert.NoError(pg.db.Unscoped().Find(&edges).Error) // find soft-deleted edges
			if test.ExpError {
				assert.Error(err)
				assert.Len(edgeedits, len(test.PreexistingEdgeEdits))
			} else {
				assert.NoError(err)
				assert.Len(edgeedits, test.ExpLenEdgeEdits)
				assert.Len(edges, test.ExpLenEdges)
			}
		})
	}
}

func TestPostgresDB_Logout(t *testing.T) {
	for _, test := range []struct {
		Name                            string
		ContextUserID, ContextAuthToken string
		PreexistingUsers                []User
		ExpError                        bool
	}{
		{
			Name: "success",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: TEST_TimeNow.Add(1 * time.Hour)}},
			}},
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
		},
		{
			Name: "fail: token invalid",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: TEST_TimeNow.Add(-1 * time.Hour)}},
			}},
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			ExpError:         true,
		},
		{
			Name:             "fail: no such user",
			ContextUserID:    "1",
			ContextAuthToken: "XXX",
			ExpError:         true,
		},
		{
			Name: "fail: no such token",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{},
			}},
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			ExpError:         true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			assert := assert.New(t)
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			err := pg.Logout(ctx)
			if test.ExpError {
				assert.Error(err)
			} else {
				assert.NoError(err)
				user := User{Model: gorm.Model{ID: 5}}
				assert.NoError(pg.db.Where(&user).First(&user).Error)
				assert.Len(user.Tokens, 0)
			}
		})
	}
}

func TestPostgresDB_DeleteAccount(t *testing.T) {
	for _, test := range []struct {
		Name                            string
		PreexistingUsers                []User
		ContextUserID, ContextAuthToken string
		ExpError                        bool
	}{
		{
			Name:             "success",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
				Tokens: []AuthenticationToken{{Token: "XXX", Expiry: TEST_TimeNow.Add(1 * time.Hour)}},
			}},
		},
		{
			Name:             "fail: no valid token",
			ContextUserID:    "5",
			ContextAuthToken: "XXX",
			PreexistingUsers: []User{{
				Model:    gorm.Model{ID: 5},
				Username: "aaaa", PasswordHash: "123", EMail: "a@b",
			}},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			assert := assert.New(t)
			for _, user := range test.PreexistingUsers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			err := pg.DeleteAccount(ctx)
			users := []User{}
			assert.NoError(pg.db.Find(&users).Error)
			if test.ExpError {
				assert.Error(err)
				assert.Len(users, 1)
			} else {
				assert.NoError(err)
				assert.Len(users, 0)
			}
		})
	}
}

func TestPostgresDB_MigrateTo(t *testing.T) {
	for _, test := range []struct {
		Name         string
		Data         db.AllData
		ExpError     bool
		ExpUsers     []User
		ExpNodes     []Node
		ExpEdges     []Edge
		ExpNodeEdits []NodeEdit
		ExpEdgeEdits []EdgeEdit
	}{
		{
			Name: "success: all the things saved",
			Data: db.AllData{
				Users: []db.User{
					{Document: db.Document{Key: "111"}, Username: "mark", PasswordHash: "1234", EMail: "mark@who",
						Tokens: []db.AuthenticationToken{{Token: "markstoken", Expiry: int64(1704103200000)}}},
				},
				Nodes: []db.Node{
					{Document: db.Document{Key: "222"}, Description: db.Text{"en": "A"}, Resources: db.Text{"en": "AAA"}},
					{Document: db.Document{Key: "333"}, Description: db.Text{"en": "B"}, Resources: db.Text{"en": "BBB"}},
				},
				Edges: []db.Edge{
					{Document: db.Document{Key: "444"}, From: "nodes/222", To: "nodes/333", Weight: 2.3},
				},
				NodeEdits: []db.NodeEdit{
					{Node: "222", User: "111", Type: db.NodeEditTypeCreate, NewNode: db.Node{Description: db.Text{"en": "A"}, Resources: db.Text{"en": "AAA"}}},
				},
				EdgeEdits: []db.EdgeEdit{
					{Edge: "444", User: "111", Type: db.EdgeEditTypeCreate, Weight: 2.3},
				},
			},
			ExpUsers: []User{
				{Model: gorm.Model{ID: 111}, Username: "mark", PasswordHash: "1234", EMail: "mark@who",
					Tokens: []AuthenticationToken{{Token: "markstoken", Expiry: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)}}},
			},
			ExpNodes: []Node{
				{Model: gorm.Model{ID: 222}, Description: db.Text{"en": "A"}, Resources: db.Text{"en": "AAA"}},
				{Model: gorm.Model{ID: 333}, Description: db.Text{"en": "B"}, Resources: db.Text{"en": "BBB"}},
			},
			ExpEdges: []Edge{
				{Model: gorm.Model{ID: 444}, FromID: 222, ToID: 333, Weight: 2.3},
			},
			ExpNodeEdits: []NodeEdit{
				{NodeID: 222, UserID: 111, Type: db.NodeEditTypeCreate, NewDescription: db.Text{"en": "A"}, NewResources: db.Text{"en": "AAA"}},
			},
			ExpEdgeEdits: []EdgeEdit{
				{EdgeID: 444, UserID: 111, Type: db.EdgeEditTypeCreate, Weight: 2.3},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := context.Background()
			assert := assert.New(t)
			err := pg.ReplaceAllDataWith(ctx, test.Data)
			if test.ExpError {
				assert.Error(err)
			} else {
				if !assert.NoError(err) {
					return
				}
			}
			users := []User{}
			assert.NoError(pg.db.Preload("Tokens").Find(&users).Error)
			for _, expUser := range test.ExpUsers {
				user := db.FindFirst(users, func(u User) bool { return u.Username == expUser.Username })
				if !assert.NotNil(user) {
					continue
				}
				assert.Equal(expUser.Username, user.Username)
				assert.Equal(expUser.EMail, user.EMail)
				assert.Equal(expUser.PasswordHash, user.PasswordHash)
				for _, expToken := range expUser.Tokens {
					token := db.FindFirst(user.Tokens, func(t AuthenticationToken) bool { return t.Token == expToken.Token })
					if !assert.NotNil(token) {
						continue
					}
					expZone, _ := expToken.Expiry.Zone()
					zone, _ := token.Expiry.Zone()
					assert.Equal(expZone, zone)
					assert.Equal(expToken.Expiry.UnixMilli(), token.Expiry.UnixMilli())
				}
			}
			nodes := []Node{}
			assert.NoError(pg.db.Find(&nodes).Error)
			for _, expNode := range test.ExpNodes {
				node := db.FindFirst(nodes, func(n Node) bool { return reflect.DeepEqual(n.Description, expNode.Description) })
				if !assert.NotNil(node) {
					continue
				}
				assert.Equal(expNode.ID, node.ID)
				assert.Equal(expNode.Resources, node.Resources)
			}
			edges := []Edge{}
			assert.NoError(pg.db.Find(&edges).Error)
			for _, expEdge := range test.ExpEdges {
				edge := db.FindFirst(edges, func(e Edge) bool { return expEdge.FromID == e.FromID && expEdge.ToID == e.ToID })
				if !assert.NotNil(edge, "\nexp:\n %v,\ngot:\n %v", test.ExpEdges, edges) {
					continue
				}
				assert.Equal(expEdge.ID, edge.ID)
				assert.Equal(expEdge.Weight, edge.Weight)
			}
			nodeedits := []NodeEdit{}
			assert.NoError(pg.db.Find(&nodeedits).Error)
			for _, expNodeEdit := range test.ExpNodeEdits {
				nodeedit := db.FindFirst(nodeedits, func(n NodeEdit) bool { return expNodeEdit.NodeID == n.NodeID /*XXX: condition not sufficient*/ })
				if !assert.NotNil(nodeedit) {
					continue
				}
				assert.Equal(expNodeEdit.NewDescription, nodeedit.NewDescription)
				assert.Equal(expNodeEdit.NewResources, nodeedit.NewResources)
				assert.Equal(expNodeEdit.Type, nodeedit.Type)
			}
			edgeedits := []EdgeEdit{}
			assert.NoError(pg.db.Find(&edgeedits).Error)
			for _, expEdgeEdit := range test.ExpEdgeEdits {
				edgeedit := db.FindFirst(edgeedits, func(e EdgeEdit) bool { return expEdgeEdit.EdgeID == e.EdgeID /*XXX: condition not sufficient*/ })
				if !assert.NotNil(edgeedit, "\nexp:\n %v,\ngot:\n %v", test.ExpEdgeEdits, edgeedits) {
					continue
				}
				assert.Equal(expEdgeEdit.Weight, edgeedit.Weight)
				assert.Equal(expEdgeEdit.Type, edgeedit.Type)
			}
		})
	}
}

func TestPostgresDB_NodeEdits(t *testing.T) {
	for _, test := range []struct {
		Name, NodeID         string
		ExpError             bool
		ExpEdits             []*model.NodeEdit
		PreexistingNodes     []Node
		PreexistingNodeEdits []NodeEdit
	}{
		{
			Name:   "created only, no edits",
			NodeID: "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, Type: db.NodeEditTypeCreate, NewDescription: db.Text{"en": "aa"}, NewResources: db.Text{"en": "RR"}},
				{NodeID: 2, UserID: 1, Type: db.NodeEditTypeCreate, NewDescription: db.Text{"en": "bb"}, NewResources: db.Text{"en": "QQ"}},
			},
			ExpEdits: []*model.NodeEdit{
				{Username: "user1", Type: model.NodeEditTypeCreate, NewDescription: "aa", NewResources: strptr("RR")},
			},
		},
		{
			Name:   "edits from multiple users",
			NodeID: "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingNodeEdits: []NodeEdit{
				{NodeID: 1, UserID: 1, NewDescription: db.Text{"en": "aa"}, Type: db.NodeEditTypeCreate},
				{NodeID: 1, UserID: 1, NewDescription: db.Text{"en": "aaa"}, Type: db.NodeEditTypeEdit},
				{NodeID: 1, UserID: 2, NewDescription: db.Text{"en": "aaaa"}, Type: db.NodeEditTypeEdit},
			},
			ExpEdits: []*model.NodeEdit{
				{Username: "user1", NewDescription: "aa", Type: model.NodeEditTypeCreate},
				{Username: "user1", NewDescription: "aaa", Type: model.NodeEditTypeEdit},
				{Username: "user2", NewDescription: "aaaa", Type: model.NodeEditTypeEdit},
			},
		},
		{
			Name:   "error: no such node",
			NodeID: "3",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := middleware.TestingCtxNewWithLanguage(context.Background(), "en")
			assert := assert.New(t)
			preexistingusers := []User{
				{Model: gorm.Model{ID: 1}, Username: "user1", PasswordHash: "000", EMail: "a@a"},
				{Model: gorm.Model{ID: 2}, Username: "user2", PasswordHash: "000", EMail: "b@b"},
			}
			for _, user := range preexistingusers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			for _, node := range test.PreexistingNodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, nodeedit := range test.PreexistingNodeEdits {
				assert.NoError(pg.db.Create(&nodeedit).Error)
			}
			edits, err := pg.NodeEdits(ctx, test.NodeID)
			if !assert.Len(edits, len(test.ExpEdits)) {
				return
			}
			for i := range test.ExpEdits {
				assert.Equal(test.ExpEdits[i].Username, edits[i].Username)
				assert.Equal(test.ExpEdits[i].Type, edits[i].Type)
				assert.Equal(test.ExpEdits[i].NewResources, edits[i].NewResources)
				assert.Equal(test.ExpEdits[i].NewDescription, edits[i].NewDescription)
				assert.True(edits[i].UpdatedAt.After(time.Now().Add(-60 * time.Minute))) // just check that it's not time.Time(0)
			}
			if test.ExpError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestPostgresDB_EdgeEdits(t *testing.T) {
	for _, test := range []struct {
		Name, EdgeID         string
		ExpError             bool
		ExpEdits             []*model.EdgeEdit
		PreexistingNodes     []Node
		PreexistingEdges     []Edge
		PreexistingEdgeEdits []EdgeEdit
	}{
		{
			Name:   "created only, no edits",
			EdgeID: "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 1, ToID: 2, Weight: 3},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate},
			},
			ExpEdits: []*model.EdgeEdit{
				{Username: "user1", Type: model.EdgeEditTypeCreate},
			},
		},
		{
			Name:   "created & 1 vote-type edit exist",
			EdgeID: "1",
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 1, ToID: 2, Weight: 3},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate, Weight: 1.0},
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeVote, Weight: 9.0},
			},
			ExpEdits: []*model.EdgeEdit{
				{Username: "user1", Type: model.EdgeEditTypeCreate, Weight: 1.0},
				{Username: "user1", Type: model.EdgeEditTypeEdit, Weight: 9.0},
			},
		},
		{
			Name:   "error: edge does not exist",
			EdgeID: "2", // 2 != Edge.ID (=1)
			PreexistingNodes: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			PreexistingEdges: []Edge{
				{Model: gorm.Model{ID: 1}, FromID: 1, ToID: 2, Weight: 3},
			},
			PreexistingEdgeEdits: []EdgeEdit{
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeCreate},
				{EdgeID: 1, UserID: 1, Type: db.EdgeEditTypeVote},
			},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			pg := setupDB(t)
			ctx := middleware.TestingCtxNewWithLanguage(context.Background(), "en")
			assert := assert.New(t)
			preexistingusers := []User{
				{Model: gorm.Model{ID: 1}, Username: "user1", PasswordHash: "000", EMail: "a@a"},
				{Model: gorm.Model{ID: 2}, Username: "user2", PasswordHash: "000", EMail: "b@b"},
			}
			for _, user := range preexistingusers {
				assert.NoError(pg.db.Create(&user).Error)
			}
			for _, node := range test.PreexistingNodes {
				assert.NoError(pg.db.Create(&node).Error)
			}
			for _, edge := range test.PreexistingEdges {
				assert.NoError(pg.db.Create(&edge).Error)
			}
			for _, edgeedit := range test.PreexistingEdgeEdits {
				assert.NoError(pg.db.Create(&edgeedit).Error)
			}
			edits, err := pg.EdgeEdits(ctx, test.EdgeID)
			if !assert.Len(edits, len(test.ExpEdits)) {
				return
			}
			for i := range test.ExpEdits {
				assert.Equal(test.ExpEdits[i].Username, edits[i].Username)
				assert.Equal(test.ExpEdits[i].Type, edits[i].Type)
				assert.Equal(test.ExpEdits[i].Weight, edits[i].Weight)
				assert.True(edits[i].UpdatedAt.After(time.Now().Add(-60 * time.Minute))) // just check that it's not time.Time(0)
			}
			if test.ExpError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

// func TestPostgresDB_(t *testing.T) {
// for _, test := range []struct {
// 	Name       string
// 	ExpError   bool
// }{
// 	{
// 		Name: "what",
// 	},
// } {
// 	t.Run(test.Name, func(t *testing.T) {
// 		pg := setupDB(t)
// 		ctx := context.Background()
// 		assert := assert.New(t)
// 		user := User{Username: "123", PasswordHash: "000", EMail: "a@b"}
// 		assert.NoError(pg.db.Create(&user).Error)
// 		currentUser := db.User{Document: db.Document{Key: itoa(user.ID)}}
// 		err := pg.?(ctx, currentUser, ?)
// 		if test.ExpError {
// 			assert.Error(err)
// 		} else {
// 			assert.NoError(err)
// 		}
// 	})
// }
// }
