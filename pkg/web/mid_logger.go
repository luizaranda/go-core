package web

import (
	"github.com/luizaranda/go-core/pkg/log"
	"net/http"
)

const (
	_requestIDHeader = "x-request-id"
	_debugHeader     = "x-debug"
)

// Logger decorates the request context with the given logger, accessible via
// the go-core log methods with context.
func Logger(logger log.Logger) Middleware {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// The scope of the variable logger is that of the parent function, which
			// on most programs will be called only once. In contrast, this function
			// will be called on a per request basis, meaning that assignment of the
			// logger variable directly causes a race condition and unexpected
			// behavior. To avoid that we assign the logger to a variable of local
			// scope, which allows us to change it without side effects.
			l := logger

			if r.Header.Get(_debugHeader) == "true" {
				l = l.WithLevel(log.DebugLevel)
			}

			if reqID := r.Header.Get(_requestIDHeader); reqID != "" {
				l = l.With(log.String("request_id", reqID))
			}

			ctx := log.Context(r.Context(), l)
			r2 := r.WithContext(ctx)

			handler(w, r2)
		}
	}
}
