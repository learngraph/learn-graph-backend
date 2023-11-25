package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/arangodb/go-driver"
	"github.com/pkg/errors"
)

// Note: cannot use []string here, as we must ensure unmarshalling creates the
// same types, same goes for the maps below
var (
	SchemaRequiredPropertiesNodes = []interface{}{"description"}
	SchemaRequiredPropertiesEdge  = []interface{}{"weight"}
	SchemaRequiredPropertiesUser  = []interface{}{"username"}

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
	"required":             SchemaRequiredPropertiesNodes,
}
var SchemaPropertyRulesEdge = map[string]interface{}{
	"properties": map[string]interface{}{
		"weight": map[string]interface{}{
			"type":             "number",
			"exclusiveMinimum": true,
			"minimum":          float64(0),
			"exclusiveMaximum": false,
			"maximum":          float64(10),
		},
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesEdge,
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
	},
	"additionalProperties": false,
	"required":             SchemaRequiredPropertiesUser,
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
			Schema: &driver.CollectionSchemaOptions{}, // <- TODO
		},
	},
	{
		Name: COLLECTION_EDGEEDITS,
		Options: driver.CreateCollectionOptions{
			Type:   driver.CollectionTypeDocument,
			Schema: &driver.CollectionSchemaOptions{}, // <- TODO
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
	_, err = db.ValidateSchema(ctx)
	return err
}
