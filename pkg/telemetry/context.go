package telemetry

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type telemetryClientCtxKey struct{}

// Helper that assigns two values to the given context, the first one with the
// providers internal key, and the second one with our own context key, allowing
// the usage of telemetry.FromContext to retrieve the telemetry.Client instance.
func contextWithTransaction(ctx context.Context, nrTX *newrelic.Transaction, client Client) context.Context {
	return Context(newrelic.NewContext(ctx, nrTX), client)
}

// Context returns a copy of the parent context in which the telemetry client
// associated with it is the one given.
//
// Usually you'll call Context with the Client returned by NewClient. Once you
// have a context with a telemetry.Client, all additional metric recording
// should be made by using the static methods exported by this package.
func Context(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, telemetryClientCtxKey{}, client)
}

// FromContext returns the telemetry.Client instance contained in a context
// via the usage of telemetry.Context function.
//
// If the context contains no client, then telemetry.DefaultTracer is returned.
func FromContext(ctx context.Context) Client {
	client, _ := ctx.Value(telemetryClientCtxKey{}).(Client)
	if client == nil {
		return DefaultTracer
	}
	return client
}
