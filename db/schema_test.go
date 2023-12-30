package db

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestEnsureSchema(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(db *MockArangoDBOperations, ctx context.Context)
		ReturnsError     bool
	}{
		{
			Name: "DB does not exist, creation successful, validation succcessful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaUnchanged, nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does not exist, creation successful, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaUnchanged, errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does not exist, creation succeeds, but open fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does not exist, creation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("database not found")).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(errors.New("singularity")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does exist, validation successful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaUnchanged, nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does exist, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaUnchanged, errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does exist, collection is missing, creation successfull, but validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(false, nil).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaUnchanged, errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "new required nodeedit field: insert current nodes",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(SchemaChangedAddNodeToEditNode, nil).Times(1)
				db.EXPECT().AddNodeToEditNode(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := context.Background()
			db := NewMockArangoDBOperations(ctrl)
			test.MockExpectations(db, ctx)
			err := EnsureSchema(db, ctx)
			if test.ReturnsError {
				assert.Error(t, err, test.Name)
			} else {
				assert.NoError(t, err, test.Name)
			}
		})
	}
}
