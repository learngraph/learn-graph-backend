package middleware

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

const contextValueLanguage = "Language"
const httpHeaderLanguage = "Language"

func AddHttp(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// TODO: add unique request id
		//log := log.Ctx(r.Context())
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

func CtxNewWithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, contextValueLanguage, lang)
}
