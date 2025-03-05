package otel

import (
	"context"
)

func Start(ctx context.Context) (ShutdownFunc, error) {
	tracingShutdownFunc, err := startTracerProvider(ctx)
	if err != nil {
		return nil, err
	}

	metricsShutdownFunc, err := startMetricsProvider(ctx)
	if err != nil {
		return nil, err
	}

	return func() error {
		if err := tracingShutdownFunc(); err != nil {
			return err
		}

		if err := metricsShutdownFunc(); err != nil {
			return err
		}

		return nil
	}, nil
}
