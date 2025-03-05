# Package telemetry

Package `telemetry` provides a set of directives for gathering metrics and tracing information in an application.
The aim of this package is to isolate the user from instrumentation provider details, presenting a single interface for all instrumentation needs.

## Quick Example

```go
func someWebHandler(w http.ResponseWriter, r *http.Request) error {
  // this will increment by one the metric some_counter.
  telemetry.Incr(r.Context(), "some_counter", telemetry.Tags("method", r.Method, "url", r.URL))

  start := time.Now()
  err := doSomeLengthyOp(r.Context())
  // The Timing directive will make a histogram with count, 95 percentile, and average on metric name time_metric.
  // Here telemetry.Tag will convert err != nil to the string "true" or "false".
  telemetry.Timing(r.Context(), "time_metric", time.Since(start), telemetry.Tags("error", err != nil))

  return err
}
```

## Metrics

`telemetry` package supports a number of metric types, including:

- Counters
- Histograms
- Distributions
- Sets
- Gauges

Each metric can be tagged by passing a slice of strings of `tag:value` to any function. Also, you can use the `telemetry.Tags` helper that will convert your tags to the string `tag:value` automatically.

## Spans

This package supports the creation of _spans_ or _segments_. Attributes can be added to spans using the `SetLabel` function.
You can nest spans by creating one using the context returned by another.

```go
func someFunction(ctx context.Context) {
  ctx, span := telemetry.StartSpan(ctx, "parent")
  defer span.Finish()
  // ...

  if someCondition {
    ctx, child := telemetry.StartSpan(ctx, "child")
    child.SetLabel("attribute", "attribute value")
    child.Finish()
 }
}
```

One thing to note is that nested spans **can't execute concurrently**. If you have to spawn a new goroutine with a nested span, you must call the `StartAsyncSpan` instead.

## Tracing

Some sort of distributed tracing support is offered by subpackage [`tracing`](./tracing) by means of header forwarding.
