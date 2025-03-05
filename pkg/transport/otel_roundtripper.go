package transport

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OpenTelemetryDecorator returns a decorator that creates a client span and injects context for distributed tracing.
// It sets OTel span status to ok if request had a response, even if it was not successful
func OpenTelemetryDecorator() RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(base)
	}
}
