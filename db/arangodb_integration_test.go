///go:build integration

package db

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/arangodb/go-driver"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

var testConfig = Config{
	Host:             "http://localhost:18529",
	NoAuthentication: true,
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

func dbTestSetupCleanup(t *testing.T) (DB, *ArangoDB, error) {
	db, err := NewArangoDB(testConfig)
	assert.NoError(t, err)
	t.Cleanup(func() { CleanupDB(db.(*ArangoDB), t) })
	SetupDB(db.(*ArangoDB), t)
	return db, db.(*ArangoDB), err
}

func TestArangoDB_CreateDBWithSchema(t *testing.T) {
	dbTestSetupCleanup(t)
}

func CreateNodesN0N1AndEdgeE0BetweenThem(t *testing.T, db *ArangoDB) {
	ctx := context.Background()
	col, err := db.db.Collection(ctx, COLLECTION_NODES)
	assert.NoError(t, err)
	meta, err := col.CreateDocument(ctx, map[string]interface{}{
		"_key":        "n0",
		"description": Text{"en": "a"},
	})
	assert.NoError(t, err, meta)
	meta, err = col.CreateDocument(ctx, map[string]interface{}{
		"_key":        "n1",
		"description": Text{"en": "b"},
	})
	assert.NoError(t, err, meta)
	col_edge, err := db.db.Collection(ctx, COLLECTION_EDGES)
	assert.NoError(t, err)
	meta, err = col_edge.CreateDocument(ctx, map[string]interface{}{
		"_key":   "e0",
		"_from":  fmt.Sprintf("%s/n0", COLLECTION_NODES),
		"_to":    fmt.Sprintf("%s/n1", COLLECTION_NODES),
		"weight": float64(3.141),
	})
	assert.NoError(t, err, meta)
}

func TestArangoDB_Graph(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, db *ArangoDB)
		ExpGraph       *model.Graph
	}{
		{
			Name: "2 nodes, no edges",
			SetupDBContent: func(t *testing.T, db *ArangoDB) {
				ctx := context.Background()
				col, err := db.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(t, err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": Text{"en": "a"},
				})
				assert.NoError(t, err, meta)
				meta, err = col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "4",
					"description": Text{"en": "b"},
				})
				assert.NoError(t, err, meta)
			},
			ExpGraph: &model.Graph{
				Nodes: []*model.Node{
					{ID: "123", Description: "a"},
					{ID: "4", Description: "b"},
				},
				Edges: nil,
			},
		},
		{
			Name:           "2 nodes, 1 edge",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			ExpGraph: &model.Graph{
				Nodes: []*model.Node{
					{ID: "n0", Description: "a"},
					{ID: "n1", Description: "b"},
				},
				Edges: []*model.Edge{
					{
						ID:     "e0",
						From:   fmt.Sprintf("%s/n0", COLLECTION_NODES),
						To:     fmt.Sprintf("%s/n1", COLLECTION_NODES),
						Weight: float64(3.141),
					},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db, d, err := dbTestSetupCleanup(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)

			// TODO: add language info as input here, don't rely on fallback to "en"
			graph, err := db.Graph(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, test.ExpGraph, graph)
		})
	}
}

func TestArangoDB_SetEdgeWeight(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, db *ArangoDB)
		EdgeID         string
		EdgeWeight     float64
		ExpErr         bool
		ExpEdge        Edge
	}{
		{
			Name:           "err: no edge with id",
			SetupDBContent: func(t *testing.T, db *ArangoDB) { /*empty db*/ },
			EdgeID:         "does-not-exist",
			ExpErr:         true,
		},
		{
			Name:           "edge found, weight changed",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			EdgeID:         "e0",
			EdgeWeight:     9.9,
			ExpErr:         false,
			ExpEdge:        Edge{Document: Document{Key: "e0"}, Weight: 9.9, From: fmt.Sprintf("%s/n0", COLLECTION_NODES), To: fmt.Sprintf("%s/n1", COLLECTION_NODES)},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db, d, err := dbTestSetupCleanup(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)
			err = db.SetEdgeWeight(context.Background(), test.EdgeID, test.EdgeWeight)
			if test.ExpErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			ctx := context.Background()
			col, err := d.db.Collection(ctx, COLLECTION_EDGES)
			assert.NoError(t, err)
			e := Edge{}
			meta, err := col.ReadDocument(ctx, "e0", &e)
			assert.NoError(t, err, meta)
			assert.Equal(t, test.ExpEdge, e)
		})
	}
}

func TestArangoDB_CreateEdge(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, db *ArangoDB)
		From, To       string
		ExpErr         bool
	}{
		{
			Name:           "err: To node-ID not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           fmt.Sprintf("%s/n0", COLLECTION_NODES), To: "does-not-exist",
			ExpErr: true,
		},
		{
			Name:           "err: From node-ID not found",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           "does-not-exist", To: fmt.Sprintf("%s/n1", COLLECTION_NODES),
			ExpErr: true,
		},
		{
			Name:           "err: edge already exists",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           fmt.Sprintf("%s/n0", COLLECTION_NODES), To: fmt.Sprintf("%s/n1", COLLECTION_NODES),
			ExpErr: true,
		},
		{
			Name:           "success: edge created and returned",
			SetupDBContent: CreateNodesN0N1AndEdgeE0BetweenThem,
			From:           fmt.Sprintf("%s/n1", COLLECTION_NODES), To: fmt.Sprintf("%s/n0", COLLECTION_NODES),
			ExpErr: false,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db, d, err := dbTestSetupCleanup(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)
			weight := 1.1
			ID, err := db.CreateEdge(context.Background(), test.From, test.To, weight)
			if test.ExpErr {
				assert.Error(t, err)
				assert.Empty(t, ID)
				return
			}
			assert.NoError(t, err)
			if !assert.NotEmpty(t, ID, "edge ID") {
				return
			}
			ctx := context.Background()
			col, err := d.db.Collection(ctx, COLLECTION_EDGES)
			assert.NoError(t, err)
			e := Edge{}
			meta, err := col.ReadDocument(ctx, ID, &e)
			assert.NoErrorf(t, err, "meta:%v,edge:%v", meta, e)
			assert.Equal(t, weight, e.Weight)
		})
	}
}

func TestArangoDB_EditNode(t *testing.T) {
	for _, test := range []struct {
		Name           string
		SetupDBContent func(t *testing.T, db *ArangoDB)
		NodeID         string
		Description    *model.Text
		ExpError       bool
		ExpDescription Text
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
			Description:    &model.Text{Translations: []*model.Translation{{Language: "en", Content: "new content"}}},
			ExpError:       false,
			ExpDescription: Text{"en": "new content"},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			db, d, err := dbTestSetupCleanup(t)
			if err != nil {
				return
			}
			test.SetupDBContent(t, d)
			ctx := context.Background()
			err = db.EditNode(ctx, test.NodeID, test.Description)
			if test.ExpError {
				assert.Error(t, err)
				return
			}
			if !assert.NoError(t, err) {
				return
			}
			col, err := d.db.Collection(ctx, COLLECTION_NODES)
			assert.NoError(t, err)
			node := Node{}
			meta, err := col.ReadDocument(ctx, test.NodeID, &node)
			assert.NoError(t, err, meta)
			assert.Equal(t, node.Description, test.ExpDescription)
		})
	}
}

func TestArangoDB_ValidateSchema(t *testing.T) {
	for _, test := range []struct {
		Name             string
		DBSetup          func(t *testing.T, db *ArangoDB)
		ExpError         bool
		ExpSchemaChanged bool
		ExpSchema        *driver.CollectionSchemaOptions
	}{
		{
			Name:             "empty db, should be NO-OP",
			DBSetup:          func(t *testing.T, db *ArangoDB) {},
			ExpSchemaChanged: false,
			ExpSchema:        &SchemaOptionsNode,
			ExpError:         false,
		},
		{
			Name: "schema correct for all entries, should be NO-OP",
			DBSetup: func(t *testing.T, db *ArangoDB) {
				ctx := context.Background()
				col, err := db.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(t, err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": Text{"en": "idk"},
				})
				assert.NoError(t, err, meta)
			},
			ExpSchemaChanged: false,
			ExpSchema:        &SchemaOptionsNode,
			ExpError:         false,
		},
		{
			Name: "schema updated (!= schema in code): new optional property -> compatible",
			DBSetup: func(t *testing.T, db *ArangoDB) {
				ctx := context.Background()
				assert := assert.New(t)
				col, err := db.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": Text{"en": "idk"},
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
			ExpSchemaChanged: true,
			ExpSchema:        &SchemaOptionsNode,
			ExpError:         false,
		},
		{
			Name: "schema updated (!= schema in code): new required property -> incompatible",
			DBSetup: func(t *testing.T, db *ArangoDB) {
				ctx := context.Background()
				assert := assert.New(t)
				col, err := db.db.Collection(ctx, COLLECTION_NODES)
				assert.NoError(err)
				meta, err := col.CreateDocument(ctx, map[string]interface{}{
					"_key":        "123",
					"description": Text{"en": "idk"},
				})
				assert.NoError(err, meta)
				props, err := col.Properties(ctx)
				assert.NoError(err)
				props.Schema.Rule = copyMap(SchemaPropertyRulesNode)
				props.Schema.Rule.(map[string]interface{})["properties"].(map[string]interface{})["newkey"] = map[string]string{
					"type": "string",
				}
				props.Schema.Rule.(map[string]interface{})["required"] = append(SchemaRequiredPropertiesNodes, "newkey")
				err = col.SetProperties(ctx, driver.SetCollectionPropertiesOptions{Schema: props.Schema})
				assert.NoError(err)
			},
			ExpSchemaChanged: true,
			ExpSchema:        nil,
			ExpError:         true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, db, err := dbTestSetupCleanup(t)
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
			col, err := db.db.Collection(ctx, COLLECTION_NODES)
			assert.NoError(err)
			props, err := col.Properties(ctx)
			assert.NoError(err)
			if test.ExpSchema != nil {
				assert.Equal(test.ExpSchema, props.Schema)
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

func TestArangoDB_CreateNode(t *testing.T) {
	for _, test := range []struct {
		Name         string
		Translations []*model.Translation
		ExpError     bool
	}{
		{
			Name: "single translation: language 'en'",
			Translations: []*model.Translation{
				{Language: "en", Content: "abc"},
			},
		},
		{
			Name: "multiple translations: language 'en', 'de', 'ch'",
			Translations: []*model.Translation{
				{Language: "en", Content: "Hello World!"},
				{Language: "de", Content: "Hallo Welt!"},
				{Language: "ch", Content: "你好世界！"},
			},
		},
		{
			Name: "invalid translation language",
			Translations: []*model.Translation{
				{Language: "AAAAA", Content: "idk"},
			},
			ExpError: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			_, db, err := dbTestSetupCleanup(t)
			if err != nil {
				return
			}
			ctx := context.Background()
			assert := assert.New(t)
			id, err := db.CreateNode(ctx, &model.Text{
				Translations: test.Translations,
			})
			if test.ExpError {
				assert.Error(err)
				assert.Equal("", id)
				return
			}
			assert.NoError(err)
			assert.NotEqual("", id)
			nodes, err := QueryReadAll[Node](ctx, db, `FOR n in nodes RETURN n`)
			assert.NoError(err)
			t.Logf("id: %v, nodes: %#v", id, nodes)
			text := Text{}
			for lang, content := range ConvertToDBText(&model.Text{Translations: test.Translations}) {
				text[lang] = content
			}
			n := FindFirst(nodes, func(n Node) bool {
				return reflect.DeepEqual(n.Description, text)
			})
			assert.NotNil(n)
		})
	}
}
