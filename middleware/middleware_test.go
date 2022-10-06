package middleware

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestAddMiddleware(t *testing.T) {
	logBuffer := bytes.NewBuffer([]byte{})
	log.Logger = zerolog.New(logBuffer).Level(zerolog.DebugLevel).With().Str("test", "test").Logger()

	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "en", CtxGetLanguage(r.Context()), "language should be set as context key")
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
