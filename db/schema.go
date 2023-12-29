package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/arangodb/go-driver"
	"github.com/pkg/errors"
)

// Note: cannot use `[]string` or `map[string]string` here, as we must ensure
// unmarshalling creates the same types.

var (
	SchemaTypeString = map[string]interface{}{"type": "string"}
)

var SchemaObjectTextTranslations = map[string]interface{}{
	"type":          "object",
	"minProperties": float64(1),
	"properties": map[string]interface{}{
		"en": SchemaTypeString,
		"de": SchemaTypeString,
		"ch": SchemaTypeString,
	},
	"additionalProperties": false,
}
var SchemaObjectAuthenticationToken = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"token":  SchemaTypeString,
		"expiry": map[string]interface{}{"type": "number"}, // , "format": "date-time"},
	},
	"required": []interface{}{"token", "expiry"},
}

var SchemaPropertyRulesNode = map[string]interface{}{
	"properties": map[string]interface{}{
		"description": SchemaObjectTextTranslations,
	},
	"additionalProperties": false,
	"required":             []interface{}{"description"},
}
var SchemaPropertyRulesNodeEdit = map[string]interface{}{
	"properties": map[string]interface{}{
		"user": SchemaTypeString,
		"node": SchemaTypeString,
		"type": map[string]interface{}{
			"type": "string",
			"enum": []NodeEditType{NodeEditTypeCreate, NodeEditTypeEdit},
		},
		"newnode": SchemaPropertyRulesNode,
	},
	"additionalProperties": false,
	"required":             []interface{}{"node", "user", "type", "newnode"},
}
var SchemaPropertyRulesEdgeWeight = map[string]interface{}{
	"type":             "number",
	"exclusiveMinimum": true,
	"minimum":          float64(0),
	"exclusiveMaximum": false,
	"maximum":          float64(10),
}
var SchemaPropertyRulesEdge = map[string]interface{}{
	"properties": map[string]interface{}{
		"weight": SchemaPropertyRulesEdgeWeight,
	},
	"additionalProperties": false,
	"required":             []interface{}{"weight"},
}
var SchemaPropertyRulesEdgeEdit = map[string]interface{}{
	"properties": map[string]interface{}{
		"user": SchemaTypeString,
		"edge": SchemaTypeString,
		"type": map[string]interface{}{
			"type": "string",
			"enum": []EdgeEditType{EdgeEditTypeCreate, EdgeEditTypeVote},
		},
		"weight": SchemaPropertyRulesEdgeWeight,
	},
	"additionalProperties": false,
	"required":             []interface{}{"edge", "user", "type"},
}
var SchemaPropertyRulesUser = map[string]interface{}{
	"properties": map[string]interface{}{
		"username":     SchemaTypeString,
		"email":        map[string]interface{}{"type": "string", "format": "email"},
		"passwordhash": SchemaTypeString,
		"authenticationtokens": map[string]interface{}{
			"type":  "array",
			"items": SchemaObjectAuthenticationToken,
		},
		// FIXME(skep): DeepEqual fails here, after retrieving the schema from ArangoDB
		// BEGIN: DeepEqual error
		"roles": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "string",
				//"enum": []string{string(RoleAdmin)}, // <- does not fix it
				"enum": []RoleType{RoleAdmin},
			},
		},
		// END: DeepEqual error
	},
	"additionalProperties": false,
	"required":             []interface{}{"username"},
}
var SchemaOptionsNode = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesNode,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesNode),
}
var SchemaOptionsEdge = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesEdge,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesEdge),
}
var SchemaOptionsUser = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesUser,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesUser),
}
var SchemaOptionsNodeEdit = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesNodeEdit,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesNodeEdit),
}
var SchemaOptionsEdgeEdit = driver.CollectionSchemaOptions{
	Rule:    SchemaPropertyRulesEdgeEdit,
	Level:   driver.CollectionSchemaLevelStrict,
	Message: fmt.Sprintf("Schema rule violated: %v", SchemaPropertyRulesEdgeEdit),
}

type IndexSpec struct {
	Property string
	Name     string
}
type CollectionSpec struct {
	Name    string
	Options driver.CreateCollectionOptions
	Indexes []IndexSpec
}

var CollectionSpecification = []CollectionSpec{
	{
		Name: COLLECTION_NODES,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeDocument,
			Schema: &SchemaOptionsNode,
		},
	},
	{
		Name: COLLECTION_EDGES,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeEdge,
			Schema: &SchemaOptionsEdge,
		},
	},
	{
		Name: COLLECTION_NODEEDITS,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeDocument,
			Schema: &SchemaOptionsNodeEdit,
		},
	},
	{
		Name: COLLECTION_EDGEEDITS,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeDocument,
			Schema: &SchemaOptionsEdgeEdit,
		},
	},
	{
		Name: COLLECTION_USERS,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeDocument,
			Schema: &SchemaOptionsUser,
		},
		Indexes: []IndexSpec{
			{Property: "email", Name: INDEX_HASH_USER_EMAIL},
			{Property: "username", Name: INDEX_HASH_USER_USERNAME},
		},
	},
}

func EnsureSchema(db ArangoDBOperations, ctx context.Context) error {
	err := db.OpenDatabase(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "database not found") {
			err2 := db.CreateDBWithSchema(ctx)
			if err2 != nil {
				return errors.Wrapf(err2, "because of %v", err)
			}
		} else {
			return err
		}
		err = db.OpenDatabase(ctx)
	}
	if err != nil {
		return err
	}
	if exists, err := db.CollectionsExist(ctx); err != nil || !exists {
		if err != nil {
			return err
		}
		if err := db.CreateDBWithSchema(ctx); err != nil {
			return err
		}
	}
	action, err := db.ValidateSchema(ctx)
	if action == SchemaChangedAddNodeToEditNode {
		db.AddNodeToEditNode(ctx)
	}
	return err
}
