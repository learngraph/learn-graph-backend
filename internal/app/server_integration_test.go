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
	//"github.com/suxatcode/learn-graph-poc-backend/db/arangodb"
	"github.com/suxatcode/learn-graph-poc-backend/db/postgres"
)

const (
	queryGraphNodeIDs = `{
  graph {
    nodes {
      id
    }
  }
}`

	queryGraphNodeAndEdgeIDs = `{
  graph {
    nodes {
      id
    }
    edges {
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

	mutationDeleteAccountWithGraphData = `` // TODO: continue with integration tests here

	mutationCreateNode = `mutation createNode ($description:Text!) {
  createNode(description:$description){
    Status {
      Message
    }
  }
}`

	mutationCreateEdge = `mutation createEdge($from: ID!, $to: ID!, $weight: Float!) {
  createEdge(from: $from, to: $to, weight: $weight) {
    ID
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

var (
	StepCreateUser = func(name string, email string) testStep {
		return testStep{
			Payload: &graphqlQuery{
				Query: mutationCreateUserWithMail,
				Variables: map[string]interface{}{
					"username": name,
					"password": "1234567890",
					"email":    email,
				},
			},
			ExpectedRegex:                   `{"data":{"createUserWithEMail":{"login":{"success":true,"token":"([^"]*)","userID":"([0-9]*)","message":null}}}}`,
			PutRegexMatchesIntoTheseHeaders: []string{"Authentication", "UserID"},
		}
	}
	StepCreateNodeOK = testStep{
		Payload: &graphqlQuery{
			Query:     mutationCreateNode,
			Variables: map[string]interface{}{"description": map[string]interface{}{"translations": []interface{}{map[string]interface{}{"language": "en", "content": "ok"}}}},
		},
		Expected: `{"data":{"createNode":{"Status":null}}}`,
	}
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
		TestSteps []testStep
	}{
		{
			Name: "query: graph",
			TestSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query: queryGraphNodeIDs,
					},
					Expected: `{"data":{"graph":{"nodes":null}}}`,
				},
			},
		},
		{
			Name: "mutation: login, expect non-existent user",
			TestSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query:     mutationUserLogin,
						Variables: map[string]interface{}{"auth": map[string]interface{}{"email": "me@ok.com", "password": "ok"}},
					},
					Expected: `{"data":{"login":{"success":false,"message":"failed to get user: record not found","token":"","userID":"","userName":""}}}`,
				},
			},
		},
		{
			Name: "mutation: deleteAccount, expect non-existent user",
			TestSteps: []testStep{
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
			Name: "mutation: createNode",
			TestSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query:     mutationCreateNode,
						Variables: map[string]interface{}{"description": map[string]interface{}{"translations": []interface{}{map[string]interface{}{"language": "en", "content": "ok"}}}},
					},
					Expected: `{"errors":[{"message":"only logged in user may create graph data","path":["createNode"]}],"data":{"createNode":null}}`,
				},
				{
					// graph should not be changed
					Payload:  &graphqlQuery{Query: queryGraphNodeIDs},
					Expected: `{"data":{"graph":{"nodes":null}}}`,
				},
			},
		},
		{
			Name: "mutation: createEdge",
			TestSteps: []testStep{
				{
					Payload: &graphqlQuery{
						Query:     mutationCreateEdge,
						Variables: map[string]interface{}{"from": "a", "to": "b", "weight": 2},
					},
					Expected: `{"errors":[{"message":"only logged in user may create graph data","path":["createEdge"]}],"data":{"createEdge":null}}`,
				},
				{
					// graph should not be changed
					Payload:  &graphqlQuery{Query: queryGraphNodeAndEdgeIDs},
					Expected: `{"data":{"graph":{"nodes":null,"edges":null}}}`,
				},
			},
		},
		{
			Name: "flow: create user, create node, query graph",
			TestSteps: []testStep{
				StepCreateUser("asdf", "a@b.co"),
				StepCreateNodeOK,
				// graph should have the new node
				{
					Payload:       &graphqlQuery{Query: queryGraphNodeIDs},
					ExpectedRegex: `{"data":{"graph":{"nodes":[{"id":"[0-9]*"}]}}}`,
				},
			},
		},
		// DISABLED
		//{
		//	Name: "flow: create user, create node, logout, create user, create node, query graph, delete all nodes by user 1 (w/ admin-key?)",
		//	TestSteps: []testStep{
		//		StepCreateUser("asdf", "a@b.co"),
		//		StepCreateNodeOK,
		//		// by creating a user we override the client headers, thus logging out of the first account
		//		StepCreateUser("qwerty", "q@w.co"),
		//		StepCreateNodeOK,
		//		// now there should be 2 nodes created by 2 different users
		//		{
		//			Payload:       &graphqlQuery{Query: queryGraphNodeIDs},
		//			ExpectedRegex: `{"data":{"graph":{"nodes":[{"id":"[0-9]*"},{"id":"[0-9]*"}]}}}`,
		//		},
		//		{
		//			Payload: &graphqlQuery{
		//				Query: mutationDeleteAccountWithGraphData,
		//				Variables: map[string]interface{}{
		//					"username": "asdf",
		//					"adminkey": "1234",
		//				},
		//			},
		//			Expected: `{"data":{"deleteAccountWithGraphData":{"Status":null}}}`,
		//		},
		//		// Note: should have exactly one node in graph now, since 2
		//		// were created, but the one from user 'asdf' was deleted with
		//		// their account.
		//		{
		//			Payload:       &graphqlQuery{Query: queryGraphNodeIDs},
		//			ExpectedRegex: `{"data":{"graph":{"nodes":[{"id":"[0-9]*"}]}}}`,
		//		},
		//	},
		//},
	} {
		t.Run(test.Name, func(t *testing.T) {
			//handler, dbtmp := graphHandler(arangodb.TESTONLY_Config)
			//arangodb.TESTONLY_SetupAndCleanup(t, dbtmp)
			handler, _ := graphHandler(postgres.TESTONLY_Config)
			postgres.TESTONLY_SetupAndCleanup(t)
			s := httptest.NewServer(handler)
			defer s.Close()
			c := s.Client()
			assert := assert.New(t)
			headers := http.Header{"Content-Type": []string{"application/json"}, "Language": []string{"en"}}
			for _, step := range test.TestSteps {
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
					continue
				}
				assert.Regexp(step.ExpectedRegex, got)
				if len(step.PutRegexMatchesIntoTheseHeaders) == 0 {
					continue
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
