package telemetry

import (
	"context"
	"time"
)

// Gauge measures the value of a metric at a particular time.
func Gauge(ctx context.Context, name string, value float64, tags []string) {
	FromContext(ctx).Gauge(name, value, tags)
}

// Count tracks how many times something happened per second.
func Count(ctx context.Context, name string, value int64, tags []string) {
	FromContext(ctx).Count(name, value, tags)
}

// Decr is just Count of -1.
func Decr(ctx context.Context, name string, tags []string) {
	FromContext(ctx).Decr(name, tags)
}

// Incr is just Count of 1.
func Incr(ctx context.Context, name string, tags []string) {
	FromContext(ctx).Incr(name, tags)
}

// Histogram tracks the statistical distribution of a set of values on each host.
func Histogram(ctx context.Context, name string, value float64, tags []string) {
	FromContext(ctx).Histogram(name, value, tags)
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func Distribution(ctx context.Context, name string, value float64, tags []string) {
	FromContext(ctx).Distribution(name, value, tags)
}

// Set counts the number of unique elements in a group.
func Set(ctx context.Context, name string, value string, tags []string) {
	FromContext(ctx).Set(name, value, tags)
}

// Timing sends timing information, it is an alias for TimeInMilliseconds.
func Timing(ctx context.Context, name string, value time.Duration, tags []string) {
	FromContext(ctx).Timing(name, value, tags)
}

// TimeInMilliseconds sends timing information in milliseconds.
// It is flushed by statsd with percentiles, mean and other info
// (https://github.com/etsy/statsd/blob/master/docs/metric_types.md#timing).
func TimeInMilliseconds(ctx context.Context, name string, value float64, tags []string) {
	FromContext(ctx).TimeInMilliseconds(name, value, tags)
}
