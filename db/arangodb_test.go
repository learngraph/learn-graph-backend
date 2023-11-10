package db

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver"
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
				db.EXPECT().ValidateSchema(ctx).Return(false, nil).Times(1)
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
				db.EXPECT().ValidateSchema(ctx).Return(false, errors.New("fail")).Times(1)
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
				db.EXPECT().ValidateSchema(ctx).Return(false, nil).Times(1)
			},
			ReturnsError: false,
		},
		{
			Name: "DB does exist, validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(true, nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(false, errors.New("fail")).Times(1)
			},
			ReturnsError: true,
		},
		{
			Name: "DB does exist, collection is missing, creation successfull, but validation fails",
			MockExpectations: func(db *MockArangoDBOperations, ctx context.Context) {
				db.EXPECT().OpenDatabase(ctx).Return(nil).Times(1)
				db.EXPECT().CollectionsExist(ctx).Return(false, nil).Times(1)
				db.EXPECT().CreateDBWithSchema(ctx).Return(nil).Times(1)
				db.EXPECT().ValidateSchema(ctx).Return(false, errors.New("fail")).Times(1)
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

func TestGetAuthentication(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Config   Config
		ExpValue string
		ExpError bool
	}{
		{
			Name: "pre-existing token",
			Config: Config{
				JwtToken: "abc",
			},
			ExpValue: "bearer abc",
		},
		{
			Name: "given secret, token must be created",
			Config: Config{
				JwtSecretPath: "./testdata/jwtSecret",
			},
			ExpValue: "bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcmFuZ29kYiIsInNlcnZlcl9pZCI6ImxlYXJuZ3JhcGgtYmFja2VuZCJ9.qVCe-sZRyu1z8Vm6zHwdgltMho0dy7UgRq6p5lttdpw",
		},
		{
			Name: "no such file at JwtSecretPath",
			Config: Config{
				JwtSecretPath: "./testdata/doesnotexist",
			},
			ExpError: true,
		},
		{
			Name: "empty file at JwtSecretPath",
			Config: Config{
				JwtSecretPath: "./testdata/emptyfile",
			},
			ExpError: true,
		},
		{
			Name: "skip authentication",
			Config: Config{
				NoAuthentication: true,
			},
			ExpError: false,
			ExpValue: "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			auth, err := GetAuthentication(test.Config)
			if test.ExpError {
				assert.Error(err)
				return
			}
			assert.NoError(err)
			if !assert.NotNil(auth) {
				return
			}
			assert.Equal(driver.AuthenticationTypeRaw, auth.Type())
			assert.Equal(test.ExpValue, auth.Get("value"))
		})
	}
}

func TestReadSecretFile(t *testing.T) {
	for _, test := range []struct {
		Name string
		File string
		Exp  string
	}{
		{
			Name: "no newline at EOF",
			File: "testdata/jwtSecret.wo-newline",
			Exp:  "b5b89b509adcf4a76ded1530d3e6c6236d0f89911f438892b2ccb992cc92371f",
		},
		{
			Name: "newline at EOF",
			File: "testdata/jwtSecret",
			Exp:  "57fe346145d78c65fe083f18a11f47f65dba3ec449f021177e6807d736360c1a",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			got, err := ReadSecretFile(test.File)
			assert.NoError(t, err)
			assert.Equal(t, test.Exp, got)
		})
	}
}
