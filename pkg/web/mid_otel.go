package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	otelcontrib "go.opentelemetry.io/contrib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	_tracerName          = "github.com/luizaranda/go-core/pkg/web"
	_instrumentationName = "github.com/luizaranda/go-core"
	_durationMetricName  = "http.server.duration"
	_unitKey             = attribute.Key("unit")
)

// responseWriter é um wrapper para http.ResponseWriter que captura o status code
type responseWriter struct {
	w      http.ResponseWriter
	status int
}

func (rw *responseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.w.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.w.WriteHeader(statusCode)
}

// Status retorna o status code armazenado
func (rw *responseWriter) Status() int {
	return rw.status
}

type OtelConfig struct {
	Propagator     propagation.TextMapPropagator
	Provider       trace.TracerProvider
	MetricProvider otelmetric.MeterProvider

	tracer         trace.Tracer
	meter          otelmetric.Meter
	durationMetric otelmetric.Int64Histogram
}

// OpenTelemetry sets up a handler to start tracing the incoming
// requests. The serverName parameter should describe the name of the
// (virtual) server handling the request.
func OpenTelemetry(cfg OtelConfig) Middleware {
	if cfg.Provider == nil {
		cfg.Provider = otel.GetTracerProvider()
	}

	if cfg.MetricProvider == nil {
		cfg.MetricProvider = otel.GetMeterProvider()
	}

	if cfg.Propagator == nil {
		cfg.Propagator = otel.GetTextMapPropagator()
	}

	cfg.tracer = cfg.Provider.Tracer(
		_tracerName,
		trace.WithInstrumentationVersion(otelcontrib.Version()),
	)

	cfg.meter = otel.Meter(_instrumentationName)
	if metric, err := cfg.meter.Int64Histogram(_durationMetricName); err == nil {
		cfg.durationMetric = metric
	}

	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			t := time.Now()

			// extract tracing header using propagator
			ctx := cfg.Propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

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

			ctx, span := cfg.tracer.Start(
				ctx, routePattern,
				trace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", r)...),
				trace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(r)...),
				trace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest("", routePattern, r)...),
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			span.SetAttributes(semconv.HTTPRouteKey.String(routePattern))
			span.SetName(routePattern)

			r2 := r.WithContext(ctx)

			// Criamos um ResponseWriter personalizado para capturar o status code
			respWriter := &responseWriter{w: w, status: http.StatusOK}
			handler(respWriter, r2)

			// set status code attribute
			status := respWriter.status
			span.SetAttributes(semconv.HTTPStatusCodeKey.Int(status))

			// set span status
			spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCode(status)
			span.SetStatus(spanStatus, spanMessage)

			// metrics middleware
			attrs := semconv.HTTPServerMetricAttributesFromHTTPRequest("", r)
			attrs = append(attrs,
				semconv.HTTPRouteKey.String(routePattern),
				semconv.HTTPStatusCodeKey.Int(status),

				// add unit to metrics attributes
				_unitKey.String("ms"),
			)

			cfg.durationMetric.Record(r.Context(), time.Since(t).Milliseconds(), otelmetric.WithAttributes(attrs...))
		}
	}
}
