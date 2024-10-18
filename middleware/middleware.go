package middleware

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	// TODO(maybe): could just use 'Accept-Language' header, with example content [en-US,en;q=0.9]
	httpHeaderLanguage = "Language"
	contextLanguage    = "Language"

	httpHeaderAuthenticationToken = "Authentication"
	contextAuthenticationToken    = "Authentication"

	httpHeaderUserID = "Userid"
	contextUserID    = "UserID"
)

func AddAll(next http.Handler) http.Handler {
	return addGlobalLoggerToReqCtx(AddUserID(AddAuthentication(AddLanguageAndLogging(next))))
}

func addGlobalLoggerToReqCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(log.Logger.WithContext(context.Background())))
	})
}

// TODO(skep): should verify the input language to be one of {en, es, de, zh, ...}, since it's a public API now!
func AddLanguageAndLogging(next http.Handler) http.Handler {
	return translateHTTPHeaderToContextValue(next, headerConfig{
		Name:       "language",
		HTTPHeader: httpHeaderLanguage,
		ContextKey: contextLanguage,
		LoggerKey:  "lang",
	})
}

func AddAuthentication(next http.Handler) http.Handler {
	return translateHTTPHeaderToContextValue(next, headerConfig{
		Name:         "authentication",
		HTTPHeader:   httpHeaderAuthenticationToken,
		ContextKey:   contextAuthenticationToken,
		RemovePrefix: "Bearer ",
	})
}

func AddUserID(next http.Handler) http.Handler {
	return translateHTTPHeaderToContextValue(next, headerConfig{
		Name:       "user ID",
		HTTPHeader: httpHeaderUserID,
		ContextKey: contextUserID,
		LoggerKey:  "userID",
	})
}

func ctxGetStringValueOrEmptyString(ctx context.Context, value string) string {
	if lang, ok := ctx.Value(value).(string); ok {
		return lang
	}
	return ""
}
func CtxGetUserID(ctx context.Context) string {
	return ctxGetStringValueOrEmptyString(ctx, contextUserID)
}
func CtxGetAuthentication(ctx context.Context) string {
	return ctxGetStringValueOrEmptyString(ctx, contextAuthenticationToken)
}
func CtxGetLanguage(ctx context.Context) string {
	return ctxGetStringValueOrEmptyString(ctx, contextLanguage)
}

// testing purposes only
func TestingCtxNewWithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, contextLanguage, lang)
}

// testing purposes only
func TestingCtxNewWithAuthentication(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, contextAuthenticationToken, token)
}

// testing purposes only
func TestingCtxNewWithUserID(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, contextUserID, token)
}

type headerConfig struct {
	Name         string
	HTTPHeader   string
	ContextKey   string
	RemovePrefix string
	// if non-empty, the HTTPHeader content will be added to *every* log output
	// done from the request context
	LoggerKey string
}

func translateHTTPHeaderToContextValue(next http.Handler, conf headerConfig) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if header, ok := r.Header[conf.HTTPHeader]; ok && len(header) == 1 {
			value := header[0]
			if conf.RemovePrefix != "" && len(value) > len(conf.RemovePrefix) && value[:len(conf.RemovePrefix)] == conf.RemovePrefix {
				value = value[len(conf.RemovePrefix):]
			}
			ctx = context.WithValue(ctx, conf.ContextKey, value)
			if conf.LoggerKey != "" {
				logger := log.Ctx(r.Context()).With().Str(conf.LoggerKey, value).Logger()
				ctx = logger.WithContext(ctx)
			}
			r = r.WithContext(ctx)
		} else {
			log.Debug().Msgf("no %s HTTP header (key='%s') found in request: %v", conf.Name, conf.HTTPHeader, r.Header)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
