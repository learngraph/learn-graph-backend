package middleware

import (
	"context"
	"log"
	"net/http"
)

const contextValueLanguage = "Language"
const httpHeaderLanguage = "Language"

func AddHttp(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// TODO: add unique request id
		log.Printf("r=%v, headers=%v", r.RemoteAddr, r.Header)
		if lang, ok := r.Header[httpHeaderLanguage]; ok && len(lang) == 1 {
			ctx := r.Context()
			ctx = context.WithValue(ctx, contextValueLanguage, lang[0])
			r = r.WithContext(ctx)
		} else {
			log.Printf("no language header ('%s') found in request", httpHeaderLanguage)
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
