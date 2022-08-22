package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGQLHandler(t *testing.T) {
	s := httptest.NewServer(graphHandler())
	defer s.Close()
	c := s.Client()
	// what :=  "?{graph{nodes{id}}}"
	payload, err := json.Marshal(
		&struct {
			Query string `json:"query"`
		}{
			Query: queryNodeIDs,
		},
	)
	if err != nil {
		t.Error(err)
		return
	}
	r, err := c.Post(s.URL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		t.Error(err)
		return
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
		return
	}
	got := string(data)
	exp := `{"data":{"graph":{"nodes":[{"id":"1"},{"id":"2"}]}}}`
	if got != exp {
		t.Errorf("got %v, but expected %v", got, exp)
	}
}

const queryNodeIDs = `{
  graph {
    nodes {
      id
    }
  }
}`
