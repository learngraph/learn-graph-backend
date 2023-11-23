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

func TestAddLanguageMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "en", CtxGetLanguage(r.Context()), "language should be set as context key")
			log.Ctx(r.Context()).Info().Msgf("AAA: headers=%v", r.Header)
		},
	)
	handler := AddLanguageAndLogging(next)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "idk", nil)
	req.Header.Add("Language", "en")
	handler.ServeHTTP(nil, req)
	assert.True(t, called, "middleware handler must call next handler")
}

func TestCtxGetLanguage(t *testing.T) {
	ctx := context.WithValue(context.Background(), "a", "c")
	assert.Equal(t, "", CtxGetLanguage(ctx), "language key not found")
	ctx = context.WithValue(context.Background(), contextLanguage, "b")
	assert.Equal(t, "b", CtxGetLanguage(ctx), "valid")
	ctx = context.WithValue(context.Background(), contextLanguage, []string{"a"})
	assert.Equal(t, "", CtxGetLanguage(ctx), "invalid type in correct key")
}

func TestAddAuthenticationMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "sometoken", CtxGetAuthentication(r.Context()), "authentication should be set as context key")
		},
	)
	handler := AddAuthentication(next)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "idk", nil)
	req.Header.Add("Authentication", "sometoken")
	handler.ServeHTTP(nil, req)
	assert.True(t, called, "middleware handler must call next handler")
}

func TestAddAuthenticationMiddlewareWithBearerPrefix(t *testing.T) {
	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "sometoken", CtxGetAuthentication(r.Context()), "authentication should be set as context key")
		},
	)
	handler := AddAuthentication(next)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "idk", nil)
	req.Header.Add("Authentication", "Bearer sometoken")
	handler.ServeHTTP(nil, req)
	assert.True(t, called, "middleware handler must call next handler")
}

func TestAddAll(t *testing.T) {
	logBuffer := bytes.NewBuffer([]byte{})
	log.Logger = zerolog.New(logBuffer).Level(zerolog.DebugLevel).With().Str("test", "test").Logger()
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token", CtxGetAuthentication(r.Context()), "auth should be in context")
			assert.Equal(t, "zh", CtxGetLanguage(r.Context()), "language should be in context")
			assert.Equal(t, "博野", CtxGetUserID(r.Context()), "user ID should be in context")
			log.Ctx(r.Context()).Info().Msg("AAA")
		},
	)
	handler := AddAll(next)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "idk", nil)
	req.Header.Add("Authentication", "token")
	req.Header.Add("Language", "zh")
	req.Header.Add("Userid", "博野")
	handler.ServeHTTP(nil, req)
	assert.Contains(t, logBuffer.String(), `AAA`)
	assert.Contains(t, logBuffer.String(), `"level":"info","test":"test"`)
	assert.Contains(t, logBuffer.String(), `"userID":"博野","lang":"zh"`)
}
