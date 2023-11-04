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
	logBuffer := bytes.NewBuffer([]byte{})
	log.Logger = zerolog.New(logBuffer).Level(zerolog.DebugLevel).With().Str("test", "test").Logger()

	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "en", CtxGetLanguage(r.Context()), "language should be set as context key")
		},
	)
	handler := AddLanguageAndLogging(next)
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

func TestAddAuthenticationMiddleware(t *testing.T) {
	logBuffer := bytes.NewBuffer([]byte{})
	log.Logger = zerolog.New(logBuffer).Level(zerolog.DebugLevel).With().Str("test", "test").Logger()

	called := false
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert.Equal(t, "sometoken", CtxGetAuthentication(r.Context()), "authentication should be set as context key")
		},
	)
	handler := AddAuthentication(next)
	handler.ServeHTTP(nil, &http.Request{
		Header: map[string][]string{
			"Authentication": {"sometoken"},
		}})
	assert.True(t, called, "middleware handler must call next handler")
	assert.Empty(t, logBuffer.String(), "log should be empty")
}

func TestAddAll(t *testing.T) {
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token", CtxGetAuthentication(r.Context()), "auth should be in context")
			assert.Equal(t, "zh", CtxGetLanguage(r.Context()), "language should be in context")
		},
	)
	handler := AddAll(next)
	handler.ServeHTTP(nil, &http.Request{
		Header: map[string][]string{
			"Authentication": {"token"},
			"Language":       {"zh"},
		}})
}
