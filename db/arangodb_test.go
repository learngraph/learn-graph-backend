package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

var testConfig = Config{
	User:     "test",
	Password: "test",
	Host:     "http://localhost:18529",
}

func TestNewArangoDB(t *testing.T) {
	_, err := NewArangoDB(testConfig)
	assert.NoError(t, err, "expected connection succeeds")
}

func TestEnsureSchema(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(db *MockArangoDBOperations, ctx context.Context)
		ReturnsError     bool
	}{
		{
			Name: "DB does not exist, creation successful, validation succcessful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does not exist, creation successful, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does not exist, creation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(os.ErrNotExist).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().OpenDatabase(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does exist, validation successful",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does exist, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
	} {
		ctrl := gomock.NewController(t)
		t.Log("Running:", test.Name)
		ctx := context.Background()
		db := NewMockArangoDBOperations(ctrl)
		test.MockExpectations(db, ctx)
		err := EnsureSchema(db, ctx)
		if test.ReturnsError {
			assert.Error(t, err, test.Name)
		} else {
			assert.NoError(t, err, test.Name)
		}
	}
}

func SetupDB(db *ArangoDB, t *testing.T) {
	err := db.CreateDBWithSchema(context.Background())
	assert.NoError(t, err)
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

func TestArangoDB_CreateDBWithSchema(t *testing.T) {
	db, err := NewArangoDB(testConfig)
	assert.NoError(t, err)
	t.Cleanup(func() { CleanupDB(db.(*ArangoDB), t) })
	SetupDB(db.(*ArangoDB), t)
}

//func TestArangoDB_Graph(t *testing.T) {
//	db, err := NewArangoDB(testConfig)
//	if err != nil {
//		t.Error("expected connection succeeds")
//	}
//	_, err = db.Graph(context.Background())
//	if err != nil {
//		t.Errorf("db.Graph: %v", err)
//	}
//}
