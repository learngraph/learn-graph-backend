//go:build integration

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
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

	mutationUserLogin = `mutation login($auth: LoginAuthentication!){
  login(authentication: $auth) {
    success
    message
    token
	userID
	userName
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

type testStep struct {
	Payload       *graphqlQuery
	Expected      string
	ExpectedRegex string
	// put matches form the response matched by ExpectedRegex into headers with
	// these names (order matters)
	PutRegexMatchesIntoTheseHeaders []string
}

func TestGraphQLHandlers(t *testing.T) {
	for _, test := range []struct {
		Name      string
		testSteps []testStep
	}{
		{
			Name: "query: graph",
			testSteps: []testStep{
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
			testSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query:     mutationUserLogin,
						Variables: map[string]interface{}{"auth": map[string]interface{}{"email": "me@ok.com", "password": "ok"}},
					},
					Expected: `{"data":{"login":{"success":false,"message":"User does not exist","token":"","userID":"","userName":""}}}`,
				},
			},
		},
		{
			Name: "mutation: deleteAccount, expect non-existent user",
			testSteps: []testStep{
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
			testSteps: []testStep{
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
			testSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query: mutationCreateUserWithMail,
						Variables: map[string]interface{}{
							"username": "asdf",
							"password": "1234567890",
							"email":    "a@b.co",
						},
					},
					ExpectedRegex:                   `{"data":{"createUserWithEMail":{"login":{"success":true,"token":"([^"]*)","userID":"([0-9]*)","message":null}}}}`,
					PutRegexMatchesIntoTheseHeaders: []string{"Authentication", "UserID"},
				},
				{
					Payload: &graphqlQuery{
						Query:     mutationCreateNode,
						Variables: map[string]interface{}{"description": map[string]interface{}{"translations": []interface{}{map[string]interface{}{"language": "en", "content": "ok"}}}},
					},
					Expected: `{"data":{"createNode":{"Status":null}}}`,
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
			headers := http.Header{"Content-Type": []string{"application/json"}}
			for _, step := range test.testSteps {
				payload, err := json.Marshal(step.Payload)
				if !assert.NoError(err) {
					return
				}
				url, err := url.Parse(s.URL)
				assert.NoError(err)
				req := http.Request{
					Method: http.MethodPost,
					Header: headers,
					URL:    url,
					Body:   io.NopCloser(strings.NewReader(string(payload))),
				}
				r, err := c.Do(&req)
				if !assert.NoError(err) {
					return
				}
				defer r.Body.Close()
				data, err := io.ReadAll(r.Body)
				if !assert.NoError(err) {
					return
				}
				got := string(data)
				if step.ExpectedRegex == "" {
					assert.Equal(step.Expected, got)
					return
				}
				assert.Regexp(step.ExpectedRegex, got)
				if len(step.PutRegexMatchesIntoTheseHeaders) == 0 {
					return
				}
				re, err := regexp.Compile(step.ExpectedRegex)
				if !assert.NoError(err) {
					return
				}
				matches := re.FindStringSubmatch(got)
				if !assert.Equal(len(step.PutRegexMatchesIntoTheseHeaders), len(matches)-1, "should find all expected matches in response") {
					return
				}
				for i, match := range matches[1:] {
					headers[step.PutRegexMatchesIntoTheseHeaders[i]] = []string{match}
				}
			}
		})
	}
}
