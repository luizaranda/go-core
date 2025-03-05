package web

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
			// create span, based on specification, we need to set already known attributes
			// when creating the span, the only thing missing here is HTTP route pattern since
			// in go-chi/chi route pattern could only be extracted once the request is executed
			// check here for details:
			//
			// https://github.com/go-chi/chi/issues/150#issuecomment-278850733
			//
			// if we have access to chi routes, we could extract the route pattern beforehand.
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
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

			// Wrap the http.ResponseWriter with a proxy for later response
			// inspection.
			w2 := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			handler(w2, r2)

			// set status code attribute
			status := w2.Status()
			span.SetAttributes(semconv.HTTPStatusCodeKey.Int(status))

			// set span status
			spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCode(status)
			span.SetStatus(spanStatus, spanMessage)

			// metrics middleware
			attrs := semconv.HTTPServerMetricAttributesFromHTTPRequest("", r)
			attrs = append(attrs,
				semconv.HTTPRouteKey.String(chi.RouteContext(r.Context()).RoutePattern()),
				semconv.HTTPStatusCodeKey.Int(status),

				// add unit to metrics attributes
				_unitKey.String("ms"),
			)

			cfg.durationMetric.Record(r.Context(), time.Since(t).Milliseconds(), otelmetric.WithAttributes(attrs...))
		}
	}
}
