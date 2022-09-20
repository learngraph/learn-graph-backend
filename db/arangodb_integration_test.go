//go:build integration

package db

import (
	"context"
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

func TestArangoDB_Graph(t *testing.T) {
	db, d, err := dbTestSetupCleanup(t)
	if err != nil {
		return
	}
	ctx := context.Background()
	col, err := d.db.Collection(ctx, COLLECTION_NODES)
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

	// TODO: add language info as input here, don't rely on fallback to "en"
	graph, err := db.Graph(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, &model.Graph{
		Nodes: []*model.Node{
			{ID: "123", Description: "a"},
			{ID: "4", Description: "b"},
		},
		Edges: nil,
	}, graph)
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
