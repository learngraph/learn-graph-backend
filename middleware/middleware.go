package middleware

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

// TODO(maybe): could just use 'Accept-Language' header, with example content [en-US,en;q=0.9]
const contextValueLanguage = "Language"
const httpHeaderLanguage = "Language"

func AddLanguageAndLogging(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// TODO: add unique request id
		log.Info().Msgf("r=%v, headers=%v", r.RemoteAddr, r.Header)
		if header, ok := r.Header[httpHeaderLanguage]; ok && len(header) == 1 {
			lang := header[0]
			logger := log.With().Str("lang", lang).Logger()
			ctx := logger.WithContext(r.Context())
			ctx = context.WithValue(ctx, contextValueLanguage, lang)
			r = r.WithContext(ctx)
		} else {
			log.Warn().Msgf("no language header ('%s') found in request", httpHeaderLanguage)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func CtxGetLanguage(ctx context.Context) string {
	if lang, ok := ctx.Value(contextValueLanguage).(string); ok {
		return lang
	}
	return ""
}

const httpHeaderAuthentication = "Authentication"
const contextAuthenticationToken = "Authentication"

func AddAuthentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if header, ok := r.Header[httpHeaderAuthentication]; ok && len(header) == 1 {
			authenticationToken := header[0]
			ctx := r.Context()
			ctx = context.WithValue(ctx, contextAuthenticationToken, authenticationToken)
			r = r.WithContext(ctx)
		} else {
			log.Warn().Msgf("no authentication header ('%s') found in request: %v", httpHeaderAuthentication, r.Header)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func CtxGetAuthentication(ctx context.Context) string {
	if lang, ok := ctx.Value(contextAuthenticationToken).(string); ok {
		return lang
	}
	return ""
}

func AddAll(next http.Handler) http.Handler {
	return AddAuthentication(AddLanguageAndLogging(next))
}
