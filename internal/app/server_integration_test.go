//go:build integration

package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
)

const (
	queryNodeIDs = `{
  graph {
    nodes {
      id
    }
  }
}`

	queryUserLogin = `query login($auth: LoginAuthentication!){
  login(authentication: $auth) {
    success
    message
    token
  }
}`

	mutationDeleteAccount = `mutation deleteAccount($user:String!){
  deleteAccount(user:$user) {
    Message
  }
}`

	mutationCreateNode = `mutation createNode ($description:Text!) {
  createNode(description:$description){
    Status {
      Message
    }
  }
}`
)

func TestGraphQLHandlers(t *testing.T) {
	for _, test := range []struct {
		Name     string
		Payload  interface{}
		Expected string
	}{
		{
			Name: "query: graph",
			Payload: &struct {
				Query string `json:"query"`
			}{
				Query: queryNodeIDs,
			},
			Expected: `{"data":{"graph":{"nodes":null}}}`,
		},
		{
			Name: "query: login, expect non-existent user",
			Payload: &struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}{
				Query:     queryUserLogin,
				Variables: map[string]interface{}{"auth": map[string]interface{}{"email": "me@ok.com", "password": "ok"}},
			},
			Expected: `{"data":{"login":{"success":false,"message":"User does not exist","token":""}}}`,
		},
		{
			Name: "mutation: deleteAccount, expect non-existent user",
			Payload: &struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}{
				Query:     mutationDeleteAccount,
				Variables: map[string]interface{}{"user": "123"},
			},
			Expected: `{"errors":[{"message":"no user with username='123' exists","path":["deleteAccount"]}],"data":{"deleteAccount":null}}`,
		},
		// FIXME(skep): This test creates a node in the test database, but does not clean up afterwards!
		//{
		//	Name: "mutation: createNode, expect success",
		//	Payload: &struct {
		//		Query     string                 `json:"query"`
		//		Variables map[string]interface{} `json:"variables"`
		//	}{
		//		Query:     mutationCreateNode,
		//		Variables: map[string]interface{}{"description": map[string]interface{}{"translations": []interface{}{map[string]interface{}{"language": "en", "content": "ok"}}}},
		//	},
		//	Expected: `{"data":{"createNode":{"Status":null}}}`,
		//},
	} {
		t.Run(test.Name, func(t *testing.T) {
			conf := db.Config{
				Host:             "http://localhost:18529",
				NoAuthentication: true,
			}
			s := httptest.NewServer(graphHandler(conf))
			defer s.Close()
			c := s.Client()
			assert := assert.New(t)
			payload, err := json.Marshal(test.Payload)
			if !assert.NoError(err) {
				return
			}
			r, err := c.Post(s.URL, "application/json", strings.NewReader(string(payload)))
			if !assert.NoError(err) {
				return
			}
			defer r.Body.Close()
			data, err := io.ReadAll(r.Body)
			if !assert.NoError(err) {
				return
			}
			got := string(data)
			assert.Equal(test.Expected, got)
		})
	}
}
