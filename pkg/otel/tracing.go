package otel

import (
	"context"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

// StartTracerProvider constructs and starts the exporter that will be sending telemetry data from a tracer provider that is set
// in a global scope for its usage.
func startTracerProvider(ctx context.Context) (ShutdownFunc, error) {
	exp, err := newTracerExporter(ctx)
	if err != nil {
		return nil, err
	}

	tp := newTracerProvider(exp)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(newPropagator())

	return func() error {
		return tp.Shutdown(ctx)
	}, nil
}

func newTracerExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	client := otlptracegrpc.NewClient(otlptracegrpc.WithEndpoint(getEndpoint()), otlptracegrpc.WithInsecure())
	return otlptrace.New(ctx, client)
}

func newTracerProvider(exp *otlptrace.Exporter) *trace.TracerProvider {
	return trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithSampler(trace.ParentBased(trace.NeverSample())),
	)
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)),
	)
}
