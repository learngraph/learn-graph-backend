//go:build integration

package arangodb

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/arangodb/go-driver"
	"github.com/google/uuid"
	"github.com/kylelemons/godebug/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
)

func init() {
	TESTONLY_initdb()
}

func testingSetupAndCleanupDB(t *testing.T) (db.DB, *ArangoDB, error) {
	db, err := NewArangoDB(TESTONLY_Config)
	assert.NoError(t, err)
	TESTONLY_SetupAndCleanup(t, db)
	return db, db.(*ArangoDB), err
}

func TestArangoDB_CreateDBWithSchema_HashIndexesOnUserCol(t *testing.T) {
	_, db, err := testingSetupAndCleanupDB(t)
	if err != nil {
		return
	}
	ctx := context.Background()
	assert := assert.New(t)
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
	assert.NoError(err)
	indexes, err := col.Indexes(ctx)
	assert.NoError(err)
	assert.Len(indexes, 3)
	index_names := []string{}
	for _, index := range indexes {
		index_names = append(index_names, index.UserName())
	}
	assert.Contains(index_names, INDEX_HASH_USER_EMAIL)
	assert.Contains(index_names, INDEX_HASH_USER_USERNAME)
}

func TestArangoDB_CreateDBWithSchema_ExistingDBButMissingCol(t *testing.T) {
	_, db, err := testingSetupAndCleanupDB(t)
	if err != nil {
		return
	}
	ctx := context.Background()
	assert := assert.New(t)
	col, err := db.db.Collection(ctx, COLLECTION_USERS)
	assert.NoError(err)
	assert.NoError(col.Remove(ctx))
	assert.NoError(db.CreateDBWithSchema(ctx))
	exists, err := db.db.CollectionExists(ctx, COLLECTION_USERS)
	assert.NoError(err)
	assert.True(exists)
}

// Note: this setup is *inconsistent*, with actual data, since no corresponding
// node/edge edit-entries exist in COLLECTION_EDGEEDITS/COLLECTION_NODEEDITS!
func CreateNodesN0N1AndEdgeE0BetweenThem(t *testing.T, adb *ArangoDB) {
	ctx := context.Background()
	col, err := adb.db.Collection(ctx, COLLECTION_NODES)
	assert.NoError(t, err)
	meta, err := col.CreateDocument(ctx, map[string]interface{}{
		"_key":        "n0",
		"description": db.Text{"en": "a"},
	})
	assert.NoError(t, err, meta)
	meta, err = col.CreateDocument(ctx, map[string]interface{}{
		"_key":        "n1",
		"description": db.Text{"en": "b"},
	})
	assert.NoError(t, err, meta)
	col_edge, err := adb.db.Collection(ctx, COLLECTION_EDGES)
	assert.NoError(t, err)
	meta, err = col_edge.CreateDocument(ctx, map[string]interface{}{
		"_key":   "e0",
		"_from":  fmt.Sprintf("%s/n0", COLLECTION_NODES),
		"_to":    fmt.Sprintf("%s/n1", COLLECTION_NODES),
		"weight": float64(2.0),
	})
	assert.NoError(t, err, meta)
}

func TestArangoDB_Graph(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, adb *ArangoDB)
		ExpGraph       *model.Graph
		Context        context.Context
	}{
		{
			Name:    "2 nodes, no edges",
			Context: middleware.TestingCtxNewWithLanguage(context.Background(), "de"),
			SetupDBContent: func(t *testing.T, adb *ArangoDB) {
				ctx := context.Background()
				col, err := adb.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(t, err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": db.Text{"de": "a"},
					"resources":   db.Text{"de": "aa"},
				})
				assert.NoError(t, err, meta)
				meta, err = col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "4",
					"description": db.Text{"de": "b"},
				})
				assert.NoError(t, err, meta)
			},
			ExpGraph: &model.Graph{
				Nodes: []*model.Node{
					{ID: "123", Description: "a", Resources: strptr("aa")},
					{ID: "4", Description: "b"},
				},
				Edges: nil,
			},
		},
		{
			Name:           "2 nodes, 1 edge",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			Context:        middleware.TestingCtxNewWithLanguage(context.Background(), "en"),
			ExpGraph: &model.Graph{
				Nodes: []*model.Node{
					{ID: "n0", Description: "a"},
					{ID: "n1", Description: "b"},
				},
				Edges: []*model.Edge{
					{
						ID:     "e0",
						From:   "n0",
						To:     "n1",
						Weight: float64(2.0),
					},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db, d, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)

			graph, err := db.Graph(test.Context)
			assert.NoError(t, err)
			assert.Equal(t, test.ExpGraph, graph)
		})
	}
}

func TestArangoDB_AddEdgeWeightVote(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		PreexistingUsers     []db.User
		PreexistingNodes     []db.Node
		PreexistingEdges     []db.Edge
		PreexistingEdgeEdits []db.EdgeEdit
		EdgeID               string
		EdgeWeight           float64
		ExpErr               bool
		ExpEdge              db.Edge
		ExpEdgeEdits         int
	}{
		{
			Name:   "err: no edge with id",
			EdgeID: "does-not-exist",
			ExpErr: true,
		},
		{
			Name: "edge found, weight averaged",
			PreexistingEdges: []db.Edge{
				{
					Document: db.Document{Key: "e0"},
					From:     fmt.Sprintf("%s/n0", COLLECTION_NODES),
					To:       fmt.Sprintf("%s/n1", COLLECTION_NODES),
					Weight:   2.0,
				},
			},
			PreexistingEdgeEdits: []db.EdgeEdit{
				{
					Edge:   "e0",
					User:   "u0",
					Type:   db.EdgeEditTypeCreate,
					Weight: 2.0,
				},
			},
			EdgeID:       "e0",
			ExpEdgeEdits: 2,
			EdgeWeight:   4.0,
			ExpErr:       false,
			ExpEdge:      db.Edge{Document: db.Document{Key: "e0"}, Weight: 3.0, From: fmt.Sprintf("%s/n0", COLLECTION_NODES), To: fmt.Sprintf("%s/n1", COLLECTION_NODES)},
		},
		{
			Name: "multiple votes exist, all shall be averaged",
			PreexistingEdges: []db.Edge{
				{
					Document: db.Document{Key: "e0"},
					From:     fmt.Sprintf("%s/n0", COLLECTION_NODES),
					To:       fmt.Sprintf("%s/n1", COLLECTION_NODES),
					Weight:   4.0,
				},
			},
			PreexistingEdgeEdits: []db.EdgeEdit{
				{
					Edge:   "e0",
					User:   "u0",
					Type:   db.EdgeEditTypeCreate,
					Weight: 2.0,
				},
				{
					Edge:   "e0",
					User:   "u0",
					Type:   db.EdgeEditTypeCreate,
					Weight: 6.0,
				},
			},
			EdgeID:       "e0",
			ExpEdgeEdits: 3,
			EdgeWeight:   10.0,
			ExpErr:       false,
			ExpEdge:      db.Edge{Document: db.Document{Key: "e0"}, Weight: 6.0, From: fmt.Sprintf("%s/n0", COLLECTION_NODES), To: fmt.Sprintf("%s/n1", COLLECTION_NODES)},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			adb, d, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, d, test.PreexistingUsers); err != nil {
				return
			}
			if err := setupDBWithGraph(t, d, test.PreexistingNodes, test.PreexistingEdges); err != nil {
				return
			}
			if err := setupDBWithEdits(t, d, []db.NodeEdit{}, test.PreexistingEdgeEdits); err != nil {
				return
			}
			err = adb.AddEdgeWeightVote(context.Background(), db.User{Document: db.Document{Key: "321"}}, test.EdgeID, test.EdgeWeight)
			assert := assert.New(t)
			if test.ExpErr {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			ctx := context.Background()
			col, err := d.db.Collection(ctx, COLLECTION_EDGES)
			assert.NoError(err)
			e := db.Edge{}
			meta, err := col.ReadDocument(ctx, "e0", &e)
			assert.NoError(err, meta)
			assert.Equal(test.ExpEdge, e)
			edgeedits, err := QueryReadAll[db.EdgeEdit](ctx, d, `FOR e in edgeedits RETURN e`)
			assert.NoError(err)
			if !assert.Len(edgeedits, test.ExpEdgeEdits) {
				return
			}
			assert.Equal(edgeedits[len(edgeedits)-1].Edge, e.Key)
			assert.Equal(edgeedits[len(edgeedits)-1].User, "321")
			assert.Equal(edgeedits[len(edgeedits)-1].Type, db.EdgeEditTypeVote)
			assert.Equal(edgeedits[len(edgeedits)-1].CreatedAt, TEST_TimeNowUnixMilli)
		})
	}
}

func TestArangoDB_CreateEdge(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, adb *ArangoDB)
		From, To       string
		ExpErr         bool
	}{
		{
			Name:           "err: 'To' node-collection not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "n0", To: "does-not-exist",
			ExpErr: true,
		},
		{
			Name:           "err: 'From' node-collection not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "does-not-exist", To: "n1",
			ExpErr: true,
		},
		{
			Name:           "err: 'From' node-ID not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "doesnotexist", To: "n1",
			ExpErr: true,
		},
		{
			Name:           "err: 'To' node-ID not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "n1", To: "doesnotexist",
			ExpErr: true,
		},
		{
			Name:           "err: edge already exists",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "n0", To: "n1",
			ExpErr: true,
		},
		{
			Name:           "err: no self-linking nodes allowed",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "n0", To: "n0",
			ExpErr: true,
		},
		{
			Name:           "success: edge created and returned",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "n1", To: "n0",
			ExpErr: false,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			adb, d, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)
			weight := 1.1
			user123 := db.User{Document: db.Document{Key: "123"}}
			ID, err := adb.CreateEdge(context.Background(), user123, test.From, test.To, weight)
			assert := assert.New(t)
			if test.ExpErr {
				assert.Error(err)
				assert.Empty(ID)
				return
			}
			assert.NoError(err)
			if !assert.NotEmptyf(ID, "edge ID: %v", err) {
				return
			}
			ctx := context.Background()
			col, err := d.db.Collection(ctx, COLLECTION_EDGES)
			assert.NoError(err)
			e := db.Edge{}
			meta, err := col.ReadDocument(ctx, ID, &e)
			if !assert.NoErrorf(err, "meta:%v,edge:%v,ID:'%s'", meta, e, ID) {
				return
			}
			assert.Equal(weight, e.Weight)
			edgeedits, err := QueryReadAll[db.EdgeEdit](ctx, d, `FOR e in edgeedits RETURN e`)
			assert.NoError(err)
			if !assert.Len(edgeedits, 1) {
				return
			}
			assert.Equal(edgeedits[0].Edge, e.Key)
			assert.Equal(edgeedits[0].User, user123.Key)
			assert.Equal(edgeedits[0].Type, db.EdgeEditTypeCreate)
			assert.Equal(edgeedits[0].Weight, weight)
			assert.Equal(edgeedits[0].CreatedAt, TEST_TimeNowUnixMilli)
		})
	}
}

func TestArangoDB_EditNode(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, adb *ArangoDB)
		NodeID         string
		Description    *model.Text
		Resources      *model.Text
		ExpError       bool
		ExpDescription db.Text
		ExpResources   db.Text
	}{
		{
			Name:           "err: node-ID not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			NodeID:         "does-not-exist",
			ExpError:       true,
		},
		{
			Name:           "success: description changed",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			NodeID:         "n0",
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "new content"},
			}},
			ExpError:       false,
			ExpDescription: db.Text{"en": "new content"},
		},
		{
			Name:           "success: description merged different languages",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			NodeID:         "n0",
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "zh", Content: "慈悲"},
			}},
			ExpError:       false,
			ExpDescription: db.Text{"en": "a", "zh": "慈悲"},
		},
		{
			Name:           "success: resources added",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			NodeID:         "n0",
			Resources: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "https://resrouce.com/en/#12"},
			}},
			ExpError:       false,
			ExpDescription: db.Text{"en": "a"},
			ExpResources:   db.Text{"en": "https://resrouce.com/en/#12"},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			adb, d, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)
			ctx := context.Background()
			err = adb.EditNode(ctx, db.User{Document: db.Document{Key: "123"}}, test.NodeID, test.Description, test.Resources)
			assert := assert.New(t)
			if test.ExpError {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			col, err := d.db.Collection(ctx, COLLECTION_NODES)
			assert.NoError(err)
			node := db.Node{}
			meta, err := col.ReadDocument(ctx, test.NodeID, &node)
			assert.NoError(err, meta)
			assert.Equal(node.Description, test.ExpDescription)
			assert.Equal(node.Resources, test.ExpResources)
			nodeedits, err := QueryReadAll[db.NodeEdit](ctx, d, `FOR e in nodeedits RETURN e`)
			assert.NoError(err)
			if !assert.Len(nodeedits, 1) {
				return
			}
			assert.Equal(test.NodeID, nodeedits[0].Node)
			assert.Equal("123", nodeedits[0].User)
			assert.Equal(db.NodeEditTypeEdit, nodeedits[0].Type)
			assert.Equal(TEST_TimeNowUnixMilli, nodeedits[0].CreatedAt)
			assert.Equal(test.ExpDescription, nodeedits[0].NewNode.Description)
		})
	}
}

func TestArangoDB_ValidateSchema(t *testing.T) {
	addNewKeyToSchema := func(propertyRules map[string]interface{}, collection string) func(t *testing.T, adb *ArangoDB) {
		return func(t *testing.T, adb *ArangoDB) {
			ctx := context.Background()
			assert := assert.New(t)
			col, err := adb.db.Collection(ctx, collection)
			assert.NoError(err)
			props, err := col.Properties(ctx)
			assert.NoError(err)
			if !assert.NotNil(props.Schema) {
				return
			}
			props.Schema.Rule = copyMap(propertyRules)
			props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{})["newkey"] = map[string]string{
				"type": "string",
			}
			err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
			assert.NoError(err)
		}
	}
	for _, test := range []struct {
		Name                   string
		DBSetup                func(t *testing.T, adb *ArangoDB)
		ExpError               bool
		ExpSchemaChanged       SchemaUpdateAction
		ExpSchema              *driver.CollectionSchemaOptions
		ExpSchemaForCollection string
	}{
		{
			Name:                   "empty db, should be NO-OP",
			DBSetup:                func(t *testing.T, adb *ArangoDB) {},
			ExpSchemaChanged:       SchemaUnchanged,
			ExpSchema:              &SchemaOptionsNode,
			ExpSchemaForCollection: COLLECTION_NODES,
			ExpError:               false,
		},
		{
			Name: "schema correct for all entries, should be NO-OP",
			DBSetup: func(t *testing.T, adb *ArangoDB) {
				ctx := context.Background()
				col, err := adb.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(t, err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": db.Text{"en": "idk"},
				})
				assert.NoError(t, err, meta)
			},
			ExpSchemaChanged:       SchemaUnchanged,
			ExpSchema:              &SchemaOptionsNode,
			ExpSchemaForCollection: COLLECTION_NODES,
			ExpError:               false,
		},
		{
			Name: "schema updated (!= schema in code): new optional property -> compatible",
			DBSetup: func(t *testing.T, adb *ArangoDB) {
				ctx := context.Background()
				assert := assert.New(t)
				col, err := adb.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": db.Text{"en": "idk"},
				})
				assert.NoError(err, meta)
				props, err := col.Properties(ctx)
				assert.NoError(err)
				props.Schema.Rule = copyMap(SchemaPropertyRulesNode)
				props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{})["newkey"] = map[string]string{
					"type": "string",
				}
				err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
				assert.NoError(err)
			},
			ExpSchemaChanged:       SchemaChangedButNoActionRequired,
			ExpSchema:              &SchemaOptionsNode,
			ExpSchemaForCollection: COLLECTION_NODES,
			ExpError:               false,
		},
		{
			Name: "schema updated (!= schema in code): new required property -> incompatible",
			DBSetup: func(t *testing.T, adb *ArangoDB) {
				ctx := context.Background()
				assert := assert.New(t)
				col, err := adb.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": db.Text{"en": "idk"},
				})
				assert.NoError(err, meta)
				props, err := col.Properties(ctx)
				assert.NoError(err)
				props.Schema.Rule = copyMap(SchemaPropertyRulesNode)
				props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{})["newkey"] = map[string]string{
					"type": "string",
				}
				props.Schema.Rule.(map[string]interface{})["required"] = append(SchemaPropertyRulesNode["required"].([]interface{}), "newkey")
				err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
				assert.NoError(err)
			},
			ExpSchemaChanged: SchemaChangedButNoActionRequired,
			ExpError:         true,
		},
		{
			Name:             "collection users should be verified",
			DBSetup:          addNewKeyToSchema(SchemaPropertyRulesUser, COLLECTION_USERS),
			ExpSchemaChanged: SchemaChangedButNoActionRequired,
			ExpError:         false,
		},
		{
			Name:             "collection nodeedits should be verified",
			DBSetup:          addNewKeyToSchema(SchemaPropertyRulesNodeEdit, COLLECTION_NODEEDITS),
			ExpSchemaChanged: SchemaChangedButNoActionRequired,
			ExpError:         false,
		},
		{
			Name:             "collection edgeedits should be verified",
			DBSetup:          addNewKeyToSchema(SchemaPropertyRulesEdgeEdit, COLLECTION_EDGEEDITS),
			ExpSchemaChanged: SchemaChangedButNoActionRequired,
			ExpError:         false,
		},
		{
			Name: "schema updated: nodes in editnode table are missing",
			DBSetup: func(t *testing.T, adb *ArangoDB) {
				ctx := context.Background()
				assert := assert.New(t)
				col, err := adb.db.Collection(ctx, COLLECTION_NODEEDITS)
				if !assert.NoError(err) {
					return
				}

				props, err := col.Properties(ctx)
				if !assert.NoError(err) {
					return
				}
				props.Schema.Rule = copyMap(props.Schema.Rule.(map[string]interface{}))
				// remove the newnode, to simmulate old DB
				delete(props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{}), "newnode")
				props.Schema.Rule.(map[string]interface{})["required"] = []interface{}{"node", "user", "type"}
				err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
				if !assert.NoError(err) {
					return
				}

				setupDBWithGraph(t, adb, []db.Node{{Document: db.Document{Key: "123"}, Description: db.Text{"en": "ok"}}}, []db.Edge{})
				_, err = col.CreateDocument(ctx, map[string]interface{}{
					"_key": "123",
					"node": "123",
					"user": "345",
					"type": "create",
				})
				if !assert.NoError(err) {
					return
				}
			},
			ExpSchemaChanged:       SchemaChangedAddNodeToEditNode,
			ExpSchema:              &SchemaOptionsNodeEdit,
			ExpSchemaForCollection: COLLECTION_NODEEDITS,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			test.DBSetup(t, db)
			ctx := context.Background()
			schemaChanged, err := db.ValidateSchema(ctx)
			assert := assert.New(t)
			assert.Equal(test.ExpSchemaChanged, schemaChanged)
			if test.ExpError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			if test.ExpSchema != nil {
				col, err := db.db.Collection(ctx, test.ExpSchemaForCollection)
				if !assert.NoError(err) {
					return
				}
				nodeProps, err := col.Properties(ctx)
				if !assert.NoError(err) {
					return
				}
				diff := pretty.Compare(test.ExpSchema, nodeProps.Schema)
				assert.Empty(diff)
			}
		})
	}
}

// recursive map copy
func copyMap(m map[string]interface{}) map[string]interface{} {
	cp := make(map[string]interface{})
	for k, v := range m {
		vm, ok := v.(map[string]interface{})
		if ok {
			cp[k] = copyMap(vm)
		} else {
			cp[k] = v
		}
	}
	return cp
}

func modelTextToDB(model *model.Text) db.Text {
	text := db.Text{}
	for lang, content := range db.ConvertToDBText(model) {
		text[lang] = content
	}
	return text
}

func TestArangoDB_CreateNode(t *testing.T) {
	for _, test := range []struct {
		Name        string
		Description *model.Text
		Resources   *model.Text
		User        db.User
		ExpError    bool
	}{
		{
			Name: "single translation: language 'en'",
			User: db.User{Document: db.Document{Key: "123"}},
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "abc"},
			}},
		},
		{
			Name: "multiple translations: language 'en', 'de', 'zh'",
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "Hello World!"},
				{Language: "de", Content: "Hallo Welt!"},
				{Language: "zh", Content: "你好世界！"},
			}},
		},
		{
			Name: "invalid translation language",
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "AAAAA", Content: "idk"},
			}},
			ExpError: true,
		},
		{
			Name: "with initial resources",
			User: db.User{Document: db.Document{Key: "123"}},
			Description: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "abc"},
			}},
			Resources: &model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "def"},
			}},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			ctx := context.Background()
			assert := assert.New(t)
			id, err := adb.CreateNode(ctx, test.User, test.Description, test.Resources)
			if test.ExpError {
				assert.Error(err)
				assert.Equal("", id)
				return
			}
			assert.NoError(err)
			assert.NotEqual("", id)
			nodes, err := QueryReadAll[db.Node](ctx, adb, `FOR n in nodes RETURN n`)
			assert.NoError(err)
			t.Logf("id: %v, nodes: %#v", id, nodes)
			description := modelTextToDB(test.Description)
			resources := modelTextToDB(test.Resources)
			n := db.FindFirst(nodes, func(n db.Node) bool {
				if test.Resources != nil {
					return reflect.DeepEqual(n.Description, description) && reflect.DeepEqual(n.Resources, resources)
				}
				return reflect.DeepEqual(n.Description, description)
			})
			if !assert.NotNil(n) {
				return
			}
			nodeedits, err := QueryReadAll[db.NodeEdit](ctx, adb, `FOR e in nodeedits RETURN e`)
			assert.NoError(err)
			if !assert.Len(nodeedits, 1) {
				return
			}
			assert.Equal(n.Key, nodeedits[0].Node)
			assert.Equal(TEST_TimeNowUnixMilli, nodeedits[0].CreatedAt)
		})
	}
}

func TestArangoDB_verifyUserInput(t *testing.T) {
	for _, test := range []struct {
		TestName      string
		User          db.User
		Password      string
		ExpectNil     bool
		Result        model.CreateUserResult
		ExistingUsers []db.User
	}{
		{
			TestName:  "duplicate username",
			User:      db.User{Username: "abcd", EMail: "abc@def.com"},
			Password:  "1234567890",
			ExpectNil: false,
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: false,
					Message: strptr("Username already exists: 'abcd'"),
				},
			},
			ExistingUsers: []db.User{
				{
					Username:     "abcd",
					EMail:        "old@mail.com",
					PasswordHash: "$2a$10$UuEBwAF9YQ2OYgTZ9qy8Oeh04HkWcC3S/P4680pz7tII.wnGc0U0y",
				},
			},
		},
		{
			TestName:  "duplicate email",
			User:      db.User{Username: "abcd", EMail: "abc@def.com"},
			Password:  "1234567890",
			ExpectNil: false,
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: false,
					Message: strptr("EMail already exists: 'abc@def.com'"),
				},
			},
			ExistingUsers: []db.User{
				{
					Username:     "mrxx",
					EMail:        "abc@def.com",
					PasswordHash: "$2a$10$UuEBwAF9YQ2OYgTZ9qy8Oeh04HkWcC3S/P4680pz7tII.wnGc0U0y",
				},
			},
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if len(test.ExistingUsers) >= 1 {
				if err := setupDBWithUsers(t, db, test.ExistingUsers); err != nil {
					return
				}
			}
			ctx := context.Background()
			assert := assert.New(t)
			res, err := db.verifyUserInput(ctx, test.User, test.Password)
			assert.NoError(err)
			if test.ExpectNil {
				assert.Nil(res)
				return
			}
			if !assert.NotNil(res) {
				return
			}
			assert.Equal(test.Result.Login.Success, res.Login.Success)
			if test.Result.Login.Message != nil {
				assert.Equal(*test.Result.Login.Message, *res.Login.Message)
			}
		})
	}
}

func TestArangoDB_createUser(t *testing.T) {
	for _, test := range []struct {
		TestName      string
		User          db.User
		Password      string
		ExpectError   bool
		Result        model.CreateUserResult
		ExistingUsers []db.User
	}{
		{
			TestName:    "username already exists",
			User:        db.User{Username: "mrxx", EMail: "new@def.com"},
			Password:    "1234567890",
			ExpectError: true,
			ExistingUsers: []db.User{
				{
					Username:     "mrxx",
					EMail:        "abc@def.com",
					PasswordHash: "$2a$10$UuEBwAF9YQ2OYgTZ9qy8Oeh04HkWcC3S/P4680pz7tII.wnGc0U0y",
				},
			},
		},
		{
			TestName: "a user with that email exists already",
			User: db.User{Username: "mrxx",
				EMail: "abc@def.com"},
			Password:    "1234567890",
			ExpectError: true,
			ExistingUsers: []db.User{
				{
					Username:     "abcd",
					EMail:        "abc@def.com",
					PasswordHash: "$2a$10$UuEBwAF9YQ2OYgTZ9qy8Oeh04HkWcC3S/P4680pz7tII.wnGc0U0y",
				},
			},
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if len(test.ExistingUsers) >= 1 {
				if err := setupDBWithUsers(t, db, test.ExistingUsers); err != nil {
					return
				}
			}
			ctx := context.Background()
			_, err = db.createUser(ctx, test.User, test.Password)
			assert := assert.New(t)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			assert.True(false) // should never be reached
		})
	}
}

func TestArangoDB_CreateUserWithEMail(t *testing.T) {
	for _, test := range []struct {
		TestName                  string
		UserName, Password, EMail string
		ExpectError               bool
		Result                    model.CreateUserResult
		ExistingUsers             []db.User
	}{
		{
			TestName: "valid everything",
			UserName: "abcd",
			Password: "1234567890",
			EMail:    "abc@def.com",
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: true,
				},
			},
		},
		{
			// MAYBE: https://github.com/wagslane/go-password-validator, or just 2FA
			TestName: "password too small: < MIN_PASSWORD_LENGTH characters",
			UserName: "abcd",
			Password: "123456789",
			EMail:    "abc@def.com",
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: false,
					Message: strptr("Password must be at least length"),
				},
			},
		},
		{
			TestName: "username too small: < MIN_USERNAME_LENGTH characters",
			UserName: "o.o",
			Password: "1234567890",
			EMail:    "abc@def.com",
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: false,
					Message: strptr("Username must be at least length"),
				},
			},
		},
		{
			TestName: "invalid email",
			UserName: "abcd",
			Password: "1234567890",
			EMail:    "abc@def@com",
			Result: model.CreateUserResult{
				Login: &model.LoginResult{
					Success: false,
					Message: strptr("Invalid EMail"),
				},
			},
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if len(test.ExistingUsers) >= 1 {
				if err := setupDBWithUsers(t, adb, test.ExistingUsers); err != nil {
					return
				}
			}
			ctx := context.Background()
			res, err := adb.CreateUserWithEMail(ctx, test.UserName, test.Password, test.EMail)
			assert := assert.New(t)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			users, err := QueryReadAll[db.User](ctx, adb, `FOR u in users RETURN u`)
			assert.NoError(err)
			if !assert.Equal(test.Result.Login.Success, res.Login.Success, "unexpected login result") {
				return
			}
			if !test.Result.Login.Success {
				assert.Contains(*res.Login.Message, *test.Result.Login.Message)
				assert.Empty(res.Login.UserID, "there should not be a user ID, if creation fails")
				assert.Empty(users, "there should be no users in DB")
				return
			}
			assert.NotEmpty(res.Login.UserID)
			if !assert.Len(users, 1, "one user should be created in DB") {
				return
			}
			assert.Equal(test.UserName, users[0].Username)
			assert.Equal(test.UserName, res.Login.UserName)
			assert.Equal(users[0].Document.Key, res.Login.UserID)
			if !assert.NotEmpty(res.Login.Token, "login token should be returned") {
				return
			}
			_, err = uuid.Parse(res.Login.Token)
			assert.NoError(err)
			assert.Len(users[0].Tokens, 1, "there should be one token in DB")
			assert.Equal(users[0].Tokens[0].Token, res.Login.Token)
		})
	}
}

func setupCollectionWithDocuments[T any](t *testing.T, adb *ArangoDB, collection string, documents []T) error {
	ctx := context.Background()
	col, err := adb.db.Collection(ctx, collection)
	if !assert.NoError(t, err) {
		return err
	}
	for _, doc := range documents {
		meta, err := col.CreateDocument(ctx, doc)
		if !assert.NoError(t, err, meta) {
			return err
		}
	}
	return nil
}

func setupDBWithUsers(t *testing.T, adb *ArangoDB, users []db.User) error {
	return setupCollectionWithDocuments(t, adb, COLLECTION_USERS, users)
}

func setupDBWithGraph(t *testing.T, adb *ArangoDB, nodes []db.Node, edges []db.Edge) error {
	err := setupCollectionWithDocuments(t, adb, COLLECTION_NODES, nodes)
	if err != nil {
		return err
	}
	return setupCollectionWithDocuments(t, adb, COLLECTION_EDGES, edges)
}

func setupDBWithEdits(t *testing.T, adb *ArangoDB, nodeedits []db.NodeEdit, edgeedits []db.EdgeEdit) error {
	err := setupCollectionWithDocuments(t, adb, COLLECTION_NODEEDITS, nodeedits)
	if err != nil {
		return err
	}
	return setupCollectionWithDocuments(t, adb, COLLECTION_EDGEEDITS, edgeedits)
}

func TestArangoDB_Login(t *testing.T) {
	for _, test := range []struct {
		TestName              string
		Auth                  model.LoginAuthentication
		ExpectError           bool
		ExpectLoginSuccess    bool
		ExpectErrorMessage    string
		ExistingUsers         []db.User
		TokenAmountAfterLogin int
		//Result                model.LoginResult
	}{
		{
			TestName: "user does not exist",
			Auth: model.LoginAuthentication{
				Email:    "abc@def.com",
				Password: "1234567890",
			},
			ExpectError:        false,
			ExpectLoginSuccess: false,
			ExpectErrorMessage: "User does not exist",
		},
		{
			TestName: "password missmatch",
			Auth: model.LoginAuthentication{
				Email:    "abc@def.com",
				Password: "1234567890",
			},
			ExpectError:        false,
			ExpectLoginSuccess: false,
			ExpectErrorMessage: "Password missmatch",
			ExistingUsers: []db.User{
				{
					Username:     "abcd",
					EMail:        "abc@def.com",
					PasswordHash: "$2a$10$UAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAI.wnGc0U0y",
				},
			},
		},
		{
			TestName: "successful login",
			Auth: model.LoginAuthentication{
				Email:    "abc@def.com",
				Password: "1234567890",
			},
			ExpectError:        false,
			ExpectLoginSuccess: true,
			ExpectErrorMessage: "",
			ExistingUsers: []db.User{
				{
					Username:     "abcd",
					EMail:        "abc@def.com",
					PasswordHash: "$2a$10$UuEBwAF9YQ2OYgTZ9qy8Oeh04HkWcC3S/P4680pz7tII.wnGc0U0y",
				},
			},
			TokenAmountAfterLogin: 1,
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if len(test.ExistingUsers) >= 1 {
				if err := setupDBWithUsers(t, adb, test.ExistingUsers); err != nil {
					return
				}
			}
			ctx := context.Background()
			res, err := adb.Login(ctx, test.Auth)
			assert := assert.New(t)
			if test.ExpectError {
				assert.Error(err)
				assert.Nil(res)
				return
			}
			if !assert.NoError(err) {
				return
			}
			assert.Equal(test.ExpectLoginSuccess, res.Success)
			if !test.ExpectLoginSuccess {
				assert.False(res.Success)
				assert.Empty(res.Token)
				assert.Contains(*res.Message, test.ExpectErrorMessage)
				return
			}
			if !assert.NotEmpty(res.Token, "login token should be returned") {
				return
			}
			assert.NotEmpty(res.UserID)
			_, err = uuid.Parse(res.Token)
			assert.NoError(err)
			users, err := QueryReadAll[db.User](ctx, adb, `FOR u in users FILTER u.email == @name RETURN u`, map[string]interface{}{
				"name": test.Auth.Email,
			})
			assert.NoError(err)
			assert.Len(users, 1)
			user := users[0]
			if !assert.Len(user.Tokens, test.TokenAmountAfterLogin) {
				return
			}
			assert.Equal(user.Tokens[test.TokenAmountAfterLogin-1].Token, res.Token)
			assert.Equal(user.Username, res.UserName)
			assert.Equal(user.Document.Key, res.UserID)
			_, err = uuid.Parse(user.Tokens[test.TokenAmountAfterLogin-1].Token)
			assert.NoError(err)
		})
	}
}

func TestArangoDB_deleteUserByKey(t *testing.T) {
	for _, test := range []struct {
		TestName, KeyToDelete string
		PreexistingUsers      []db.User
		MakeCtxFn             func(ctx context.Context) context.Context
		ExpectError           bool
	}{
		{
			TestName:    "successful deletion",
			KeyToDelete: "123",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "123"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "TOKEN"},
					},
				},
			},
			MakeCtxFn: func(ctx context.Context) context.Context {
				return middleware.TestingCtxNewWithAuthentication(ctx, "TOKEN")
			},
		},
		{
			TestName:    "error: token expired",
			KeyToDelete: "123",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "123"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(-24 * time.Hour).UnixMilli(), Token: "TOKEN"},
					},
				},
			},
			MakeCtxFn: func(ctx context.Context) context.Context {
				return middleware.TestingCtxNewWithAuthentication(ctx, "TOKEN")
			},
			ExpectError: true,
		},
		{
			TestName:    "error: no such user ID",
			KeyToDelete: "1",
			ExpectError: true,
		},
		{
			TestName:    "error: no matching auth token for user ID",
			KeyToDelete: "123",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "123"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Token: "AAAAA"},
					},
				},
			},
			MakeCtxFn: func(ctx context.Context) context.Context {
				return middleware.TestingCtxNewWithAuthentication(ctx, "BBBBBB")
			},
			ExpectError: true,
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, adb, test.PreexistingUsers); err != nil {
				return
			}
			ctx := context.Background()
			if test.MakeCtxFn != nil {
				ctx = test.MakeCtxFn(ctx)
			}
			assert := assert.New(t)
			err = adb.deleteUserByKey(ctx, test.KeyToDelete)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
		})
	}
}

var ralf = db.User{
	Document:     db.Document{Key: "123"},
	Username:     "ralf",
	EMail:        "a@b.com",
	PasswordHash: "321",
	Tokens: []db.AuthenticationToken{
		{Token: "TOKEN"},
	},
}

func TestArangoDB_getUserByProperty(t *testing.T) {
	for _, test := range []struct {
		TestName, Property, Value string
		PreexistingUsers          []db.User
		ExpectError               bool
		ExpectedResult            *db.User
	}{
		{
			TestName: "retrieve existing user by username successfully",
			Property: "username",
			Value:    "ralf",
			PreexistingUsers: []db.User{
				ralf,
			},
			ExpectedResult: &ralf,
		},
		{
			TestName:         "error: no such user",
			Property:         "username",
			Value:            "ralf",
			PreexistingUsers: []db.User{},
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, db, test.PreexistingUsers); err != nil {
				return
			}
			ctx := context.Background()
			user, err := db.getUserByProperty(ctx, test.Property, test.Value)
			assert := assert.New(t)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			assert.Equal(test.ExpectedResult, user)
		})
	}
}

func TestArangoDB_DeleteAccount(t *testing.T) {
	for _, test := range []struct {
		TestName         string
		PreexistingUsers []db.User
		MakeCtxFn        func(ctx context.Context) context.Context
		ExpectError      bool
	}{
		{
			TestName: "successful deletion",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "abcd"},
					Username:     "lmas",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "TOKEN"},
					},
				},
			},
			MakeCtxFn: func(ctx context.Context) context.Context {
				ctx = middleware.TestingCtxNewWithAuthentication(ctx, "TOKEN")
				ctx = middleware.TestingCtxNewWithUserID(ctx, "abcd")
				return ctx
			},
		},
		{
			TestName: "error: no such user for _key",
			MakeCtxFn: func(ctx context.Context) context.Context {
				return middleware.TestingCtxNewWithUserID(ctx, "abcd")
			},
			ExpectError: true,
		},
		{
			TestName: "error: no matching auth token for user._key",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "abcd"},
					Username:     "lmas",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "AAAAA"},
					},
				},
			},
			MakeCtxFn: func(ctx context.Context) context.Context {
				ctx = middleware.TestingCtxNewWithAuthentication(ctx, "BBBBBB")
				ctx = middleware.TestingCtxNewWithUserID(ctx, "abcd")
				return ctx
			},
			ExpectError: true,
		},
	} {
		t.Run(test.TestName, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, db, test.PreexistingUsers); err != nil {
				return
			}
			ctx := context.Background()
			if test.MakeCtxFn != nil {
				ctx = test.MakeCtxFn(ctx)
			}
			assert := assert.New(t)
			err = db.DeleteAccount(ctx)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
		})
	}
}

func TestArangoDB_Logout(t *testing.T) {
	for _, test := range []struct {
		Name                            string
		ContextUserID, ContextAuthToken string
		PreexistingUsers                []db.User
		ExpErr                          bool
		ExpTokenLenAfterLogout          int
	}{
		{
			Name:             "successful logout",
			ContextUserID:    "123",
			ContextAuthToken: "TOKEN",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "123"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "TOKEN"},
					},
				},
			},
			ExpErr:                 false,
			ExpTokenLenAfterLogout: 0,
		},
		{
			Name:             "fail: token missmatch",
			ContextUserID:    "456",
			ContextAuthToken: "AAA",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "456"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "BBB"},
					},
				},
			},
			ExpErr:                 true,
			ExpTokenLenAfterLogout: 1,
		},
		{
			Name:             "success: token expired, but doesn't matter user wants to remove it anyways",
			ContextUserID:    "456",
			ContextAuthToken: "TOKEN",
			PreexistingUsers: []db.User{
				{
					Document:     db.Document{Key: "456"},
					Username:     "abcd",
					EMail:        "a@b.com",
					PasswordHash: "321",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(-24 * time.Hour).UnixMilli(), Token: "TOKEN"},
					},
				},
			},
			ExpErr:                 false,
			ExpTokenLenAfterLogout: 0,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, adb, test.PreexistingUsers); err != nil {
				return
			}
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			assert := assert.New(t)
			err = adb.Logout(ctx)
			if test.ExpErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			users, err := QueryReadAll[db.User](ctx, adb, `FOR u in users FILTER u._key == @key RETURN u`, map[string]interface{}{
				"key": test.ContextUserID,
			})
			if len(users) != 1 {
				return
			}
			assert.NoError(err)
			assert.Len(users[0].Tokens, test.ExpTokenLenAfterLogout)
		})
	}
}

func TestArangoDB_IsUserAuthenticated(t *testing.T) {
	for _, test := range []struct {
		Name                            string
		ContextUserID, ContextAuthToken string
		PreexistingUsers                []db.User
		ExpErr                          bool
		ExpAuth                         bool
	}{
		{
			Name:             "userID not found",
			ContextUserID:    "qwerty",
			ContextAuthToken: "123",
			ExpErr:           false,
			ExpAuth:          false,
		},
		{
			Name:             "userID found, but no valid token",
			ContextUserID:    "qwerty",
			ContextAuthToken: "AAA",
			PreexistingUsers: []db.User{
				{
					Document: db.Document{Key: "qwerty"},
					Username: "asdf",
					EMail:    "a@b.com",
					Tokens: []db.AuthenticationToken{
						{Token: "BBB"},
					},
				},
			},
			ExpErr:  false,
			ExpAuth: false,
		},
		{
			Name:             "user authenticated, everything valid",
			ContextUserID:    "qwerty",
			ContextAuthToken: "AAA",
			PreexistingUsers: []db.User{
				{
					Document: db.Document{Key: "qwerty"},
					Username: "abcd",
					EMail:    "a@b.com",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "AAA"},
					},
				},
			},
			ExpErr:  false,
			ExpAuth: true,
		},
		{
			Name:             "user authenticated, matching token, but expired",
			ContextUserID:    "qwerty",
			ContextAuthToken: "AAA",
			PreexistingUsers: []db.User{
				{
					Document: db.Document{Key: "qwerty"},
					Username: "abcd",
					EMail:    "a@b.com",
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(-24 * time.Hour).UnixMilli(), Token: "AAA"},
					},
				},
			},
			ExpErr:  false,
			ExpAuth: false,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, db, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, db, test.PreexistingUsers); err != nil {
				return
			}
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			auth, user, err := db.IsUserAuthenticated(ctx)
			assert := assert.New(t)
			assert.Equal(test.ExpAuth, auth)
			if test.ExpErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
			if test.ExpAuth {
				assert.NotNil(user)
			}
		})
	}
}

func TestArangoDB_DeleteAccountWithData(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		UsernameToDelete     string
		UsersLeftOver        int
		ContextUserID        string
		ContextAuthToken     string
		ExpectError          bool
		PreexistingUsers     []db.User
		PreexistingNodes     []db.Node
		PreexistingNodeEdits []db.NodeEdit
		PreexistingEdges     []db.Edge
		PreexistingEdgeEdits []db.EdgeEdit
	}{
		{
			Name:             "deletion of single node",
			UsernameToDelete: "asdf",
			UsersLeftOver:    1,
			ContextUserID:    "hasadmin",
			ContextAuthToken: "AAA",
			PreexistingUsers: []db.User{
				{
					Document: db.Document{Key: "1"},
					Username: "asdf",
					EMail:    "a@b.com",
				},
				{
					Document: db.Document{Key: "hasadmin"},
					Username: "qwerty",
					EMail:    "d@e.com",
					Roles:    []db.RoleType{db.RoleAdmin},
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "AAA"},
					},
				},
			},
			PreexistingNodes: []db.Node{
				{
					Document:    db.Document{Key: "2"},
					Description: db.Text{"en": "hello"},
				},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{
					Document: db.Document{Key: "3"},
					Node:     "2",
					User:     "1",
					Type:     db.NodeEditTypeCreate,
					NewNode: db.Node{
						Document:    db.Document{Key: "2"},
						Description: db.Text{"en": "hello"},
					},
				},
			},
		},
		{
			Name:             "user has no admin role -> expect failure",
			UsernameToDelete: "asdf",
			UsersLeftOver:    1,
			ContextUserID:    "2",
			ContextAuthToken: "AAA",
			ExpectError:      true,
			PreexistingUsers: []db.User{
				{
					Document: db.Document{Key: "1"},
					Username: "asdf",
					EMail:    "a@b.com",
				},
				{
					Document: db.Document{Key: "2"},
					Username: "qwerty",
					EMail:    "d@e.com",
					Roles:    []db.RoleType{ /*empty!*/ },
					Tokens: []db.AuthenticationToken{
						{Expiry: TEST_TimeNow.Add(24 * time.Hour).UnixMilli(), Token: "AAA"},
					},
				},
			},
			PreexistingNodes: []db.Node{
				{
					Document:    db.Document{Key: "2"},
					Description: db.Text{"en": "hello"},
				},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{
					Document: db.Document{Key: "3"},
					Node:     "2",
					User:     "1",
					Type:     db.NodeEditTypeCreate,
					NewNode: db.Node{
						Document:    db.Document{Key: "2"},
						Description: db.Text{"en": "hello"},
					},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithUsers(t, adb, test.PreexistingUsers); err != nil {
				return
			}
			if err := setupDBWithGraph(t, adb, test.PreexistingNodes, test.PreexistingEdges); err != nil {
				return
			}
			if err := setupDBWithEdits(t, adb, test.PreexistingNodeEdits, test.PreexistingEdgeEdits); err != nil {
				return
			}
			ctx := middleware.TestingCtxNewWithUserID(context.Background(), test.ContextUserID)
			ctx = middleware.TestingCtxNewWithAuthentication(ctx, test.ContextAuthToken)
			err = adb.DeleteAccountWithData(ctx, test.UsernameToDelete, "1234")
			assert := assert.New(t)
			if test.ExpectError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			users, err := QueryReadAll[db.User](ctx, adb, `FOR u in users RETURN u`)
			assert.NoError(err)
			assert.Len(users, test.UsersLeftOver)
		})
	}
}

func TestArangoDB_AddNodeToEditNode(t *testing.T) {
	for _, test := range []struct {
		Name               string
		NodeEditsOldSchema []map[string]interface{}
		NodeEditsNewSchema []map[string]interface{}
		ExpNodeEdits       []db.NodeEdit
		Nodes              []db.Node
	}{
		{
			Name: "change single nodeedit entry",
			NodeEditsOldSchema: []map[string]interface{}{
				{
					"_key":       "111",
					"node":       "222",
					"user":       "333",
					"type":       db.NodeEditTypeCreate,
					"created_at": TEST_TimeNowUnixMilli,
				},
			},
			Nodes: []db.Node{
				{Document: db.Document{Key: "222"}, Description: db.Text{"en": "ok"}},
			},
			ExpNodeEdits: []db.NodeEdit{
				{
					Document: db.Document{Key: "111"}, Node: "222", User: "333", Type: db.NodeEditTypeCreate,
					NewNode:   db.Node{Document: db.Document{Key: "222"}, Description: db.Text{"en": "ok"}},
					CreatedAt: TEST_TimeNowUnixMilli,
				},
			},
		},
		{
			Name: "change one out of two nodeedit entries",
			NodeEditsOldSchema: []map[string]interface{}{
				{
					"_key":       "111",
					"node":       "222",
					"user":       "333",
					"type":       db.NodeEditTypeCreate,
					"created_at": TEST_TimeNowUnixMilli,
				},
			},
			NodeEditsNewSchema: []map[string]interface{}{
				{
					"_key":       "444",
					"node":       "555",
					"user":       "333",
					"type":       db.NodeEditTypeEdit,
					"created_at": TEST_TimeNowUnixMilli,
					"newnode": map[string]interface{}{
						"_key":        "555",
						"description": map[string]interface{}{"en": "SHOULD_NOT_BE_CHANGED"},
					},
				},
			},
			Nodes: []db.Node{
				{Document: db.Document{Key: "222"}, Description: db.Text{"en": "ok"}},
				{Document: db.Document{Key: "555"}, Description: db.Text{"en": "nok"}},
			},
			ExpNodeEdits: []db.NodeEdit{
				{
					Document: db.Document{Key: "111"},
					Node:     "222", User: "333", Type: db.NodeEditTypeCreate,
					NewNode:   db.Node{Document: db.Document{Key: "222"}, Description: db.Text{"en": "ok"}},
					CreatedAt: TEST_TimeNowUnixMilli,
				},
				{
					Document: db.Document{Key: "444"},
					Node:     "555", User: "333", Type: db.NodeEditTypeEdit,
					NewNode:   db.Node{Document: db.Document{Key: "555"}, Description: db.Text{"en": "SHOULD_NOT_BE_CHANGED"}},
					CreatedAt: TEST_TimeNowUnixMilli,
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			ctx := context.Background()
			assert := assert.New(t)
			col, err := adb.db.Collection(ctx, COLLECTION_NODEEDITS)
			if !assert.NoError(err) {
				return
			}
			props, err := col.Properties(ctx)
			if !assert.NoError(err) {
				return
			}
			// apply old schema to be able to insert inconsistent data
			props.Schema.Rule = copyMap(props.Schema.Rule.(map[string]interface{}))
			delete(props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{}), "newnode")
			props.Schema.Rule.(map[string]interface{})["required"] = []interface{}{"node", "user", "type"}
			err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
			if !assert.NoError(err) {
				return
			}
			setupDBWithGraph(t, adb, test.Nodes, []db.Edge{})
			for _, nodeedit := range test.NodeEditsOldSchema {
				_, err = col.CreateDocument(ctx, nodeedit)
				if !assert.NoError(err) {
					return
				}
			}
			// apply current schema again, so that the AddNodeToEditNode code can fix it
			props.Schema.Rule = SchemaPropertyRulesNodeEdit
			err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
			if !assert.NoError(err) {
				return
			}
			for _, nodeedit := range test.NodeEditsNewSchema {
				_, err = col.CreateDocument(ctx, nodeedit)
				if !assert.NoError(err) {
					return
				}
			}
			adb.AddNodeToEditNode(ctx)
			nodeedits, err := QueryReadAll[db.NodeEdit](ctx, adb, `FOR e in nodeedits RETURN e`)
			assert.Len(nodeedits, len(test.ExpNodeEdits))
			less := func(i, j int) bool { return strings.Compare(nodeedits[i].Key, nodeedits[j].Key) <= 0 }
			sort.Slice(nodeedits, less)
			sort.Slice(test.ExpNodeEdits, less)
			assert.Equal(test.ExpNodeEdits, nodeedits)
		})
	}
}

func TestArangoDB_DeleteNode(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		PreexistingNodes     []db.Node
		PreexistingEdges     []db.Edge
		PreexistingNodeEdits []db.NodeEdit
		ExpError             bool
	}{
		{
			Name: "node has multiple edits -> err",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "123"}, Description: db.Text{"en": "ok"}},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{Node: "123", User: "aaa", Type: db.NodeEditTypeEdit, NewNode: db.Node{Description: db.Text{"en": "ok"}}},
				{Node: "123", User: "uuu", Type: db.NodeEditTypeCreate, NewNode: db.Node{Description: db.Text{"en": "A"}}},
			},
			ExpError: true,
		},
		{
			Name: "node has no edit, created by another user -> err",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "123"}, Description: db.Text{"en": "ok"}},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{Node: "123", User: "aaa", Type: db.NodeEditTypeEdit, NewNode: db.Node{Description: db.Text{"en": "ok"}}},
			},
			ExpError: true,
		},
		{
			Name: "node has no edit, created by this user: perform delete",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "123"}, Description: db.Text{"en": "ok"}},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{Node: "123", User: "uuu", Type: db.NodeEditTypeEdit, NewNode: db.Node{Description: db.Text{"en": "ok"}}},
			},
		},
		{
			Name: "link to the node exists -> err",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "123"}, Description: db.Text{"en": "ok"}},
				{Document: db.Document{Key: "222"}, Description: db.Text{"en": "nok"}},
			},
			PreexistingNodeEdits: []db.NodeEdit{
				{Node: "123", User: "uuu", Type: db.NodeEditTypeEdit, NewNode: db.Node{Description: db.Text{"en": "ok"}}},
			},
			PreexistingEdges: []db.Edge{
				{From: "nodes/123", To: "nodes/222", Weight: 3.3},
			},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithGraph(t, adb, test.PreexistingNodes, test.PreexistingEdges); err != nil {
				return
			}
			if err := setupDBWithEdits(t, adb, test.PreexistingNodeEdits, []db.EdgeEdit{}); err != nil {
				return
			}
			ctx := context.Background()
			assert := assert.New(t)
			err = adb.DeleteNode(ctx, db.User{Document: db.Document{Key: "uuu"}}, "123")
			if test.ExpError {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			nodeedits, err := QueryReadAll[db.NodeEdit](ctx, adb, `FOR e in nodeedits RETURN e`)
			if !assert.NoError(err) {
				return
			}
			assert.Len(nodeedits, 0)
			nodes, err := QueryReadAll[db.Node](ctx, adb, `FOR n in nodes RETURN n`)
			if !assert.NoError(err) {
				return
			}
			assert.Len(nodes, 0)
		})
	}
}

func TestArangoDB_DeleteEdge(t *testing.T) {
	for _, test := range []struct {
		Name                 string
		PreexistingNodes     []db.Node
		PreexistingEdges     []db.Edge
		PreexistingEdgeEdits []db.EdgeEdit
		ExpError             bool
	}{
		{
			Name: "node has multiple edits -> err",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "1"}, Description: db.Text{"en": "ok1"}},
				{Document: db.Document{Key: "2"}, Description: db.Text{"en": "ok2"}},
			},
			PreexistingEdges: []db.Edge{
				{Document: db.Document{Key: "123"}, From: "nodes/1", To: "nodes/2", Weight: 3.3},
			},
			PreexistingEdgeEdits: []db.EdgeEdit{
				{Edge: "123", User: "aaa", Type: db.EdgeEditTypeVote, Weight: 1.1},
				{Edge: "123", User: "uuu", Type: db.EdgeEditTypeCreate, Weight: 9.9},
			},
			ExpError: true,
		},
		{
			Name: "node has no edit, created by another user -> err",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "1"}, Description: db.Text{"en": "ok1"}},
				{Document: db.Document{Key: "2"}, Description: db.Text{"en": "ok2"}},
			},
			PreexistingEdges: []db.Edge{
				{Document: db.Document{Key: "123"}, From: "nodes/1", To: "nodes/2", Weight: 3.3},
			},
			PreexistingEdgeEdits: []db.EdgeEdit{
				{Edge: "123", User: "aaa", Type: db.EdgeEditTypeCreate, Weight: 1.1},
			},
			ExpError: true,
		},
		{
			Name: "node has no edit, created by this user: perform delete",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "1"}, Description: db.Text{"en": "ok1"}},
				{Document: db.Document{Key: "2"}, Description: db.Text{"en": "ok2"}},
			},
			PreexistingEdges: []db.Edge{
				{Document: db.Document{Key: "123"}, From: "nodes/1", To: "nodes/2", Weight: 3.3},
			},
			PreexistingEdgeEdits: []db.EdgeEdit{
				{Edge: "123", User: "uuu", Type: db.EdgeEditTypeCreate, Weight: 1.1},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithGraph(t, adb, test.PreexistingNodes, test.PreexistingEdges); err != nil {
				return
			}
			if err := setupDBWithEdits(t, adb, []db.NodeEdit{}, test.PreexistingEdgeEdits); err != nil {
				return
			}
			ctx := context.Background()
			assert := assert.New(t)
			err = adb.DeleteEdge(ctx, db.User{Document: db.Document{Key: "uuu"}}, "123")
			if test.ExpError {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			edgeedits, err := QueryReadAll[db.EdgeEdit](ctx, adb, `FOR e in edgeedits RETURN e`)
			if !assert.NoError(err) {
				return
			}
			assert.Len(edgeedits, 0)
			edges, err := QueryReadAll[db.Edge](ctx, adb, `FOR n in edges RETURN n`)
			if !assert.NoError(err) {
				return
			}
			assert.Len(edges, 0)
		})
	}
}

func TestArangoDB_Node(t *testing.T) {
	for _, test := range []struct {
		Name             string
		NodeID           string
		PreexistingNodes []db.Node
		PreexistingEdges []db.Edge
		ExpError         bool
		ExpNode          *model.Node
	}{
		{
			Name:   "success: empty resources",
			NodeID: "1",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "1"}, Description: db.Text{"en": "n1"}},
			},
			ExpNode: &model.Node{
				ID:          "1",
				Description: "n1",
				Resources:   nil,
			},
		},
		{
			Name:   "success: full node",
			NodeID: "1",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "1"}, Description: db.Text{"en": "n1"}, Resources: db.Text{"en": "https://some.link/#n1"}},
			},
			ExpNode: &model.Node{
				ID:          "1",
				Description: "n1",
				Resources:   strptr("https://some.link/#n1"),
			},
		},
		{
			Name:   "error: node not found",
			NodeID: "1",
			PreexistingNodes: []db.Node{
				{Document: db.Document{Key: "2"}, Description: db.Text{"en": "n2"}},
			},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, adb, err := testingSetupAndCleanupDB(t)
			if err != nil {
				return
			}
			if err := setupDBWithGraph(t, adb, test.PreexistingNodes, test.PreexistingEdges); err != nil {
				return
			}
			assert := assert.New(t)
			ctx := middleware.TestingCtxNewWithLanguage(context.Background(), "en")
			node, err := adb.Node(ctx, test.NodeID)
			if test.ExpError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			assert.Equal(test.ExpNode, node)
		})
	}
}
