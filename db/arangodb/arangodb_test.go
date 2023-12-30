package arangodb

import (
	"testing"

	"github.com/arangodb/go-driver"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
)

func TestGetAuthentication(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Config   db.Config
		ExpValue string
		ExpError bool
	}{
		{
			Name: "pre-existing token",
			Config: db.Config{
				JwtToken: "abc",
			},
			ExpValue: "bearer abc",
		},
		{
			Name: "given secret, token must be created",
			Config: db.Config{
				JwtSecretPath: "./testdata/jwtSecret",
			},
			ExpValue: "bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhcmFuZ29kYiIsInNlcnZlcl9pZCI6ImxlYXJuZ3JhcGgtYmFja2VuZCJ9.qVCe-sZRyu1z8Vm6zHwdgltMho0dy7UgRq6p5lttdpw",
		},
		{
			Name: "no such file at JwtSecretPath",
			Config: db.Config{
				JwtSecretPath: "./testdata/doesnotexist",
			},
			ExpError: true,
		},
		{
			Name: "empty file at JwtSecretPath",
			Config: db.Config{
				JwtSecretPath: "./testdata/emptyfile",
			},
			ExpError: true,
		},
		{
			Name: "skip authentication",
			Config: db.Config{
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
