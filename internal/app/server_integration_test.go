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

	queryUserLogin = `mutation login($auth: LoginAuthentication!){
  login(authentication: $auth) {
    success
    message
    token
  }
}`

	mutationDeleteAccount = `mutation deleteAccount {
  deleteAccount {
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

	mutationCreateUserWithMail = `mutation createUserWithEMail($username: String!, $password: String!, $email: String!) {
  createUserWithEMail(username: $username, password: $password, email: $email) {
    login {
      success
	  token
	  userID
      message
    }
  }
}`
)

type graphqlQuery struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type queryAndResult struct {
	Payload       *graphqlQuery
	Expected      string
	ExpectedRegex string
}

func TestGraphQLHandlers(t *testing.T) {
	for _, test := range []struct {
		Name          string
		QuerySequence []queryAndResult
	}{
		{
			Name: "query: graph",
			QuerySequence: []queryAndResult{
				{
					Payload: &graphqlQuery{
						Query: queryNodeIDs,
					},
					Expected: `{"data":{"graph":{"nodes":null}}}`,
				},
			},
		},
		{
			Name: "mutation: login, expect non-existent user",
			QuerySequence: []queryAndResult{
				{
					Payload: &graphqlQuery{
						Query:     queryUserLogin,
						Variables: map[string]interface{}{"auth": map[string]interface{}{"email": "me@ok.com", "password": "ok"}},
					},
					Expected: `{"data":{"login":{"success":false,"message":"User does not exist","token":""}}}`,
				},
			},
		},
		{
			Name: "mutation: deleteAccount, expect non-existent user",
			QuerySequence: []queryAndResult{
				{
					Payload: &graphqlQuery{
						Query:     mutationDeleteAccount,
						Variables: map[string]interface{}{"user": "123"},
					},
					Expected: `{"errors":[{"message":"no userID in HTTP-header found","path":["deleteAccount"]}],"data":{"deleteAccount":null}}`,
				},
			},
		},
		{
			Name: "mutation: createNode", // TODO(skep): expect failure, only logged in users may create graph data",
			QuerySequence: []queryAndResult{
				{
					Payload: &graphqlQuery{
						Query:     mutationCreateNode,
						Variables: map[string]interface{}{"description": map[string]interface{}{"translations": []interface{}{map[string]interface{}{"language": "en", "content": "ok"}}}},
					},
					Expected: `{"data":{"createNode":{"Status":null}}}`,
					//Expected: `{"errors":[{"message":"only logged in user may create graph data","path":["createNode"]}],"data":{"createNode":null}}`,
				},
			},
		},
		{
			Name: "flow: create user, 2x create node, create edge, query graph",
			QuerySequence: []queryAndResult{
				{
					Payload: &graphqlQuery{
						Query: mutationCreateUserWithMail,
						Variables: map[string]interface{}{
							"username": "asdf",
							"password": "1234567890",
							"email":    "a@b.co",
						},
					},
					ExpectedRegex: `{"data":{"createUserWithEMail":{"login":{"success":true,"token":"[^"]*","userID":"[0-9]*","message":null}}}}`,
				},
			},
		},
		//{
		//	Name: "flow: create user, create node, logout, create user, create node, query graph, delete all nodes by user 1 (w/ admin-key)",
		//	QuerySequence: []queryAndResult{ },
		//},
	} {
		t.Run(test.Name, func(t *testing.T) {
			handler, dbtmp := graphHandler(db.TESTONLY_Config)
			//arangodb := dbtmp.(*db.ArangoDB)
			db.TESTONLY_SetupAndCleanup(t, dbtmp)
			s := httptest.NewServer(handler)
			defer s.Close()
			c := s.Client()
			assert := assert.New(t)
			for _, testQuery := range test.QuerySequence {
				payload, err := json.Marshal(testQuery.Payload)
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
				if testQuery.ExpectedRegex != "" {
					assert.Regexp(testQuery.ExpectedRegex, got)
				} else {
					assert.Equal(testQuery.Expected, got)
				}
			}
		})
	}
}
