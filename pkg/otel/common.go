package otel

import (
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
)

const (
	_defaultAgentHost = "otel-agent"
	_defaultAgentPort = "4317"

	_otelAgentHostEnv = "OTEL_HOST"
	_otelAgentPortEnv = "OTEL_PORT"
)

// ShutdownFunc for shutting down the tracer provider and its components.
type ShutdownFunc func() error

func getEndpoint() string {
	host := os.Getenv(_otelAgentHostEnv)
	if host == "" {
		host = _defaultAgentHost
	}
	port := os.Getenv(_otelAgentPortEnv)
	if port == "" {
		port = _defaultAgentPort
	}
	return fmt.Sprintf("%s:%s", host, port)
}

func setOTelDefaults() {
	otel.SetTracerProvider(nil)
	otel.SetTextMapPropagator(nil)
	otel.SetMeterProvider(nil)
}
