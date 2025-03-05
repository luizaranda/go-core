package web

import (
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
	"net/http"
)

// HeaderForwarder decorates a request context with the value of certain headers
// in order to allow transport.HTTPRequester to use those headers in outgoing requests.
func HeaderForwarder() Middleware {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r2 := r.WithContext(tracing.ContextFromHTTPHeader(r.Context(), r.Header))
			handler(w, r2)
		}
	}
}
