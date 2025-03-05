package web

import (
	"context"
	"net/http"
)

// UncancellableContext stops a request context cancellation being propagated to
// its child contexts.
//
// The normal operation is for a request context to be canceled when the
// incoming request net.Conn is ended. This is the best default behavior as we
// can avoid doing extra work when there's no-one on the other side to receive
// the response.
func UncancellableContext() Middleware {
	return func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithoutCancel(r.Context())

			r2 := r.WithContext(ctx)
			h(w, r2)
		}
	}
}
