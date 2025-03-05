package web

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/luizaranda/go-core/pkg/log"
	"github.com/luizaranda/go-core/pkg/telemetry"
)

// Panics handles any panic that may occur by notifying the error to an external system such as DataDOG or NewRelic
// and responding to the client with a status code 500.
// For this middleware to log, it requires the context to have a log.Logger.
func Panics() Middleware {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					err, ok := rvr.(error)
					if !ok {
						err = fmt.Errorf("%v", rvr)
					}

					log.Error(r.Context(), "panic recover", log.Err(err))

					routePattern := chi.RouteContext(r.Context()).RoutePattern()
					tags := []string{
						"method:" + r.Method,
						"handler:" + telemetry.SanitizeMetricTagValue(routePattern),
					}
					telemetry.Incr(r.Context(), "toolkit.http.server.panic_recovered", tags)

					notifyErr(r.Context(), err)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()

			handler(w, r)
		}
	}
}
