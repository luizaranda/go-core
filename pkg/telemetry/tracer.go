package telemetry

import (
	"context"
	"net/http"
	"time"
)

type Client interface {
	Close() error
	StartSpan(ctx context.Context, name string) (context.Context, Span)
	StartWebSpan(ctx context.Context, name string, w http.ResponseWriter, r *http.Request) (context.Context, Span)
	Gauge(name string, value float64, tags []string)
	Count(name string, value int64, tags []string)
	Incr(name string, tags []string)
	Decr(name string, tags []string)
	Histogram(name string, value float64, tags []string)
	Distribution(name string, value float64, tags []string)
	Set(name string, value string, tags []string)
	Timing(name string, value time.Duration, tags []string)
	TimeInMilliseconds(name string, value float64, tags []string)
}
