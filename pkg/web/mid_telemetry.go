package web

import (
	"fmt"
	"github.com/luizaranda/go-core/pkg/telemetry"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Telemetry middleware simplifies tracing of incoming web requests by
// initiating a new Span and composing the request context with it.
// It also records different metrics such as:
// - Count of requests per handler by {method,status}
// - Timing of response per handler by {method,status}.
func Telemetry(tracer telemetry.Client) Middleware {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Tenta obter o contexto Gin, se disponível
			var routePattern string

			// Verifica se o contexto Gin está disponível no request
			if gc, exists := r.Context().Value(gin.ContextKey).(*gin.Context); exists && gc != nil {
				// No Gin, o padrão de rota é obtido através do FullPath()
				routePattern = gc.FullPath()
			} else {
				// Fallback para compatibilidade
				routePattern = r.URL.Path
			}

			// New Relic instrumentation
			txName := fmt.Sprintf("%s (%s)", routePattern, r.Method)

			ctx, span := tracer.StartWebSpan(r.Context(), txName, w, r)
			defer span.Finish()

			// The Span returned by tracer.StartWebSpan may be a
			// http.ResponseWriter as well. In this case we want to use it when
			// calling the user handler.
			spanWriter, ok := span.(http.ResponseWriter)
			if ok {
				w = spanWriter
			}

			r2 := r.WithContext(ctx)

			// Criamos um ResponseWriter personalizado para capturar o status code
			w2 := &responseWriter{w: w, status: http.StatusOK}

			start := time.Now()
			handler(w2, r2)
			recordRequest(tracer, w2.Status(), time.Since(start), r.Method, routePattern)
		}
	}
}

func recordRequest(tracer telemetry.Client, status int, delta time.Duration, method, routePattern string) {
	// If client skips writing the header, the standard library will default to status code 200 OK.
	// https://github.com/golang/go/blob/go1.16/src/net/http/server.go#L1625
	if status == 0 {
		status = 200
	}

	tags := []string{
		"status:" + strconv.Itoa(status),
		"status_class:" + strconv.Itoa(status/100) + "xx", // 2xx, 3xx, 4xx, 5xx
		"method:" + method,
		"handler:" + telemetry.SanitizeMetricTagValue(routePattern),
	}

	tracer.Incr("toolkit.http.server.request", tags)
	tracer.Timing("toolkit.http.server.request.time", delta, tags)
}
