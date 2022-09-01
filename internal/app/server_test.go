package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
		},
	)
	handler := addMiddleware(next)
	handler.ServeHTTP(nil, &http.Request{})
	assert.True(t, called, "middleware handler must called next handler")
}
