package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
)

const (
	_collectTimeout  = 35 * time.Second
	_collectPeriod   = 30 * time.Second
	_minimumInterval = time.Minute
)

var _histogramBuckets = []float64{5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000, 25000, 50000, 100000}

// StartMetricsProvider constructs and starts the exporter that will be sending telemetry data from a tracer provider that is set
// in a global scope for its usage.
func startMetricsProvider(ctx context.Context) (ShutdownFunc, error) {
	exp, err := newMetricExporter(ctx)
	if err != nil {
		return nil, err
	}

	mp := newMeterProvider(exp)
	otel.SetMeterProvider(mp)

	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(_minimumInterval))
	if err != nil {
		return nil, err
	}

	return func() error {
		return mp.Shutdown(ctx)
	}, nil
}

func newMetricExporter(ctx context.Context) (metric.Exporter, error) {
	return otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(getEndpoint()), otlpmetricgrpc.WithInsecure())
}

func newMeterProvider(metricExporter metric.Exporter) *metric.MeterProvider {
	// This new factory is to redefine the histograms buckets, because the default values are few and very low
	return metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(
				metricExporter,
				metric.WithTimeout(_collectTimeout),
				metric.WithInterval(_collectPeriod))),
		metric.WithView(metric.NewView(
			metric.Instrument{
				Name: "*",
				Kind: metric.InstrumentKindHistogram,
			},
			metric.Stream{
				Aggregation: metric.AggregationExplicitBucketHistogram{
					Boundaries: _histogramBuckets,
				},
			},
		)),
	)
}
