//go:build integration

package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGraphHandler(t *testing.T) {
	os.Setenv("DB_ARANGO_HOST", "http://localhost:18529")
	s := httptest.NewServer(graphHandler())
	defer s.Close()
	c := s.Client()
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
	exp := `{"data":{"graph":{"nodes":null}}}`
	assert.Equal(t, exp, got)
}

const queryNodeIDs = `{
  graph {
    nodes {
      id
    }
  }
}`
