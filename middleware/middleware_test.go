package middleware

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddMiddleware(t *testing.T) {
	logBuffer := bytes.NewBuffer([]byte{})
	log.SetOutput(logBuffer)
	defer log.SetOutput(os.Stderr)
	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "en", CtxGetLanguage(r.Context()))
		},
	)
	handler := AddHttp(next)
	handler.ServeHTTP(nil, &http.Request{
		Header: map[string][]string{
			"Language": {"en"},
		}})
	assert.True(t, called, "middleware handler must call next handler")
	assert.Contains(t, logBuffer.String(), "r=, headers=map[Language:[en]]", "log should contain request & headers")
}

func TestCtxGetLanguage(t *testing.T) {
	ctx := context.WithValue(context.Background(), "a", "c")
	assert.Equal(t, "", CtxGetLanguage(ctx), "language key not found")
	ctx = context.WithValue(context.Background(), contextValueLanguage, "b")
	assert.Equal(t, "b", CtxGetLanguage(ctx), "valid")
	ctx = context.WithValue(context.Background(), contextValueLanguage, []string{"a"})
	assert.Equal(t, "", CtxGetLanguage(ctx), "invalid type in correct key")
}
