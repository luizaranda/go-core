# Package transport

Package `transport` implements some missing functionality from the standard library HTTP components in the most generic way possible (as
`http.RoundTripper`) allowing users to easily implement them on their HTTP Client.

> ⚠️ Unless you are building an `http.Client` from scratch we recommend you use the functionality provided by `transport` through the
[`transport/httpclient`](./transport/httpclient/README.md) package.

## Creating a Transport

The standard library `http.Transport` implementation is the component responsible for effectively executing HTTP requests. The zero value
for `http.Transport` and `http.DefaultTransport` are not suitable for production use. The timeout values are set in the seconds. Likewise
the dial methods used to establish TCP connections are allowed up to thirty seconds before failing.

Instantiating an `http.Transport` and `net.Dialer` with sane values can be daunting, to help with this `transport` package offers a way of
building a `http.Transport` using functional arguments that simplify the modification of timeout values, also providing sane (though
opinionated) defaults for timeouts.

```go
opts := []transport.Option{
  OptionDialTimeout(timeout time.Duration),
  OptionResponseHeaderTimeout(timeout time.Duration),
  OptionExpectContinueTimeout(timeout time.Duration),
  OptionIdleConnTimeout(timeout time.Duration),
  OptionTLSHandshakeTimeout(timeout time.Duration),
  OptionTLSClientConfig(config *tls.Config),
}

client := http.Client{
  Transport: trasport.NewTransport(opts...),
}
```

## Transport Pool Stats

One of the responsibilities of the `http.Transport` component is to provide TCP connection pooling. One thing lacking though is being able
to query the state of that pool.

This package provides a `transport.PooledTransport` type which wraps the standard library `http.Transport` and provides a `Stats()` method
which returns the number of established connection for each network address.

Stats for a `PooledTransport` can also be accessed through [`expvar`](https://godoc.org/expvar) under the key `toolkit.http.client.conn_pools`.

Build a pooled transport from scratch, using the same options as `transport.NewTransport`:

```go
opts := []transport.Option{
  OptionDialTimeout(timeout time.Duration),
  OptionResponseHeaderTimeout(timeout time.Duration),
  ...
}

tr := NewPooled("uniquePoolName", opts...)
client := http.Client{
  Transport: tr,
}
```

Build a `PooledTransport` from an existing `http.Transport`:

```go
tr := http.Transport{}
tr2 := NewPooledFromTransport("uniquePoolName", &tr)
client := &http.Client{
  Transport: tr2,
}
```

## RoundTrippers

### Hook RoundTripper

A `HookRoundTripper` provides a way to setup hooks to be called before executing an HTTP Request and after the response is received. Request
hooks are allowed to return an error which results in no request being executed.

```go
client := &http.Client{
  Transport: &transport.HookRoundTripper{
    // The underlying transport actually used to make requests
    Transport: http.DefaultTransport,

    // RequestHook allows a user-supplied function to be called
    // before each retry.
    RequestHook: []transport.RequestHook{...},

    // ResponseHook allows a user-supplied function to be called
    // with the response from each HTTP request executed.
    ResponseHook: []transport.ResponseHook{...},
  },
}
```

### Traced RoundTripper

> Tag `target_id` is retrieved from the `http.Request` context. If not present then the tag is avoided. Refer to
[`telemetry/tracing`](/pkg/telemetry/tracing) for more information on `target_id`.

For any distributed application it's fundamental that HTTP requests performed by it are traced. This RoundTripper provides an opinionated
implementation of traceability. For each executed request it records the following metrics:

| NAME                                         | TYPE           | APPLICABLE TO | TAGS                                                                                                               | DESCRIPTION                                                                                                                                                   |
|----------------------------------------------|----------------|---------------|--------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| toolkit.http.client.dns.time                 | Histogram (ms) | NEW CONNS     | target_id:string, status:(ok,error,timeout)                                                                        | Time between start of DNS query and end.                                                                                                                      |
| toolkit.http.client.tcp_connect.time         | Histogram (ms) | NEW CONNS     | target_id:string, status:(ok,error,timeout)                                                                        | Time between start of TCP conn dial and end.                                                                                                                  |
| toolkit.http.client.tls_handshake.time       | Histogram (ms) | NEW CONNS     | target_id:string, status:(ok,error,timeout)                                                                        | Time between start of TLS handshake (if applicable) and end.                                                                                                  |
| toolkit.http.client.got_connection.time      | Histogram (ms) | ALL           | target_id:string, reused:bool                                                                                      | Request connection obtained, either by establishing a new one or getting one from the pool. Connections from the pool may have a wait time, this is measured. |
| toolkit.http.client.request_written.time     | Histogram (ms) | ALL           | target_id:string, method:(get,post,etc), reused:bool, status:(ok,error,timeout)                                    | Time between the start of the request until request is fully written into socket.                                                                             |
| toolkit.http.client.response_first_byte.time | Histogram (ms) | ALL           | target_id:string, method:(get,post,etc), reused:bool                                                               | Time between the start of the request until response first byte is read from socket.                                                                          |
| toolkit.http.client.request.time             | Histogram (ms) | ALL           | target_id:string, method:(get,post,etc), reused:bool, status:(int,error,timeout), status_class:(2xx,3xx,etc,error) | Time between the start of the request until response headers are fully read from socket.                                                                      |
| toolkit.http.client.response_fully_read.time | Histogram (ms) | ALL           | target_id:string, method:(get,post,etc), reused:bool, status:(int,error,timeout), status_class:(2xx,3xx,etc,error) | Time between the start of the request until response body is fully read from socket.                                                                          | 

These metrics are not gathered by default (except for `toolkit.http.client.request.time`). If you wish to activate these low level metrics, you should initialize your httpclient with the option `WithEnableClientTrace`.

Some metrics try to distinguish the cause of an error (if any). If an error occur, then `status` is either `error` or `timeout`. To
decide whether status should be `timeout`, we call `os.IsTimeout` function with the error, if true then `status:timeout` is used
instead of `status:error`. Keep this in mind as under what circumstances `os.IsTimeout` returns true may change between Go versions.

This RoundTripper also wraps the one from NewRelic providing automatic tracking of requests as external segments if the request context
contains a NewRelic Transaction. The ExternalSegment "Procedure" is set to  `endpoint_template` or `target_id` (in that order) from the
request context (more information in [`telemetry/tracing`](/pkg/telemetry/tracing)).

### Target RoundTripper

> This `RoundTripper` provides helper functionality for tracing purposes. For more information on `target_id` refer to
[`telemetry/tracing`](/pkg/telemetry/tracing).

When adding telemetry to HTTP requests it's important to be able to distinguish requests for different servers or endpoints.
`TargetRoundTripper` allows setting a default `target_id` for every request that does not already contain one.

### Circuit Breaker RoundTripper

CircuitBreakerRoundTripper simplifies the process of using a circuit breaker at the transport level. Requests are
mapped into a bucket which is then passed into the Circuit Breaker to see if it's allowed or not.

The interface required by this RoundTripper is satisfied by the `BucketBreaker` type present in the
[`breaker`](/pkg/breaker) package.
