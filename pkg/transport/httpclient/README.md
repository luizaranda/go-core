# Package httpclient

Package `httpclient` provides users with a friendlier API for creating HTTP Requesters than the standard library does.
Unlike the standard library's http.Client implementation, an `httpclient` is production-ready with sane defaults.

It also provides additional functionality by leveraging the components in [`transport`](/pkg/transport/README.md)
package, and extending them with additional types.

By using the `New` or `NewRetryable` methods for instantiating an HTTP client it ensures you will have:

- Connection pooling by a using the same `http.Transport`.
- Sane default timeouts (300ms for TCP dialing, 3s as request timeout).
- Tracing Header forwarding as implemented by [`pkg/telemetry/tracing`](/pkg/telemetry/tracing).
- Outgoing header indicating if a request is produced in response to a retry.
- HTTP's traceability as implemented by `TracedRoundTripper` in [`transport`](/pkg/transport/README.md) package.

Optional parameters include:

- `DisableTimeout`: Disabled request timeout, dial timeout is still present unless a transport with no dial timeout is given.
- `WithTimeout`: Allows configuring the request timeout.
- `FollowRedirects`: Tells the HTTP client whether it must follow redirects or not. Default is to not follow them.
- `EnableCache`: Enables HTTP caching of responses. By default, it uses a shared local memory cache of 500MB. All Requesters share the same
cache unless a different one is given.
- `WithCache`: Allows setting a custom storage backend for HTTP caching.
- `WithCircuitBreaker`: Allows setting a circuit breaker which will be sent the targetID of the request in order to check
whether it should be performed or not.
- `WithEnableClientTrace`: Allows to gather low level metrics for troubleshooting. These metrics are the following:
  * `toolkit.http.client.dns.time`
  * `toolkit.http.client.tcp_connect.time`
  * `toolkit.http.client.tls_handshake.time`
  * `toolkit.http.client.got_connection.time`
  * `toolkit.http.client.request_written.time`
  * `toolkit.http.client.response_first_byte.time`
  * `toolkit.http.client.response_fully_read.time`
- `WithRequestHook`: Allows settings hooks to be executed before each request.
- `WithResponseHook`: Allows setting hooks to be executed after each response is received, but before returning control to the caller.
- `WithTransport`: Allows building a Requester with a custom `transport.PooledTransport`.

Optional parameters for `NewRetryable`:

- `WithRetryPolicy`: Allows customizing the retry policy. Default policy `HTTPRetryPolicy` retries on server errors and timeouts.
- `WithBackoffStrategy`: Allows customizing the retry backoff strategy, `ConstantBackoff` and `ExponentialBackoff` are already
provided.

## Usage

> ⚠️ **NOTE**: For a higher level abstraction of REST APIs we recommend using [`rusty`](/pkg/rusty/README.md) which simplifies the usage of
the `Requester` and helps avoid some of its gotchas.

A Requester is a type that can execute requests. We wanted to be compatible with the standard library signature, which results in a
requester being:

```go
// Requester exposes the http.Client.Do method, which is the minimum
// required method for executing HTTP requests.
type Requester interface {
  Do(*http.Request) (*http.Response, error)
}
```

This package implements different variations of `Requester`. The underlying types used for implementing them are exported for advanced users
that need additional tweaking or do not want some of the opinionated defaults.

> ⚠️ **IMPORTANT**: Semantics of the returned `Requester` are the same as the ones of `http.Client`. For example, the request timeout still
counts towards reading the response body; the TCP connection is not returned to the pool if the response body is not read and closed, etc.

### Simple Requester

This Requester is a decorated standard library `http.Client`. It's instantiated by calling `New`. It performs an HTTP
request and returns its response, in the same way as a plain `http.Client` would.

```go
opts := []httpclient.Option{
  httpclient.WithTimeout(5*time.Second),
}
client := httpclient.New(opts...)

req, err := http.NewRequestWithContext(context.TODO(), "GET", "http://www.google.com/", nil)
if err != nil {
  return err
}

res, err := client.Do(req)
if err != nil {
  return err // transport error
}

// assert res status code
res.Status

// read response body
body, err := ioutil.ReadAll(res.Body)
if err != nil {
  return err // unable to read whole response body, possible request timeout or TCP error.
}
res.Body.Close() // close body after reading it so that TCP connection is released back into the pool.
```

### Retryable Requesters

Sometimes a remote server has some error rate or intermittent issues we want to avoid; this can also occur at the network level, with some
of our requests taking to long to process. One strategy used to avoid the impact of this issues is to retry requests.

This Requester is a decorated `httpclient.RetryableClient`. It's instantiated by calling `NewRetryable`. It performs an HTTP request,
and retries it under error.

> ⚠️ **IMPORTANT**: Retrying requests is not always safe. You **must** understand the impact on the remote server to know if it's safe to
do a retry. A good rule of thumb is that `GET/OPTIONS/HEAD` requests are usually safe to retry, while `POST/PUT/DELETE` are not.
Configuring the correct Backoff strategy is crucial, failure to do so may result catastrophic results for the target API.

Using the snippet of the simple request, just change the instantiation to:

```go
maxRetries := 3 // Number of maximum retries to do. This means that a maximum total of 4 requests may execute (the original + 3 retries).
client := httpclient.NewRetryable(maxRetries, opts...)
```

> ⚠️ **IMPORTANT**: When retrying requests with body the client needs to re-read the body in order to retry it. Doing this on an `io.Reader`
which is what `http.Request` provides may be expensive depending on the underlying type. In order to optimize this we provide a way of
instantiating an `http.Request` that results in more optimal resource usage. So, when creating requests with body use
`httpclient.NewRequest` instead of standard library `http.NewRequestWithContext`.

When the `RetryableClient` retries a request, it records the following metric:

| NAME                                    | TYPE    | TAGS                                                                                                  |
|-----------------------------------------|---------|-------------------------------------------------------------------------------------------------------|
| toolkit.http.client.request.retry.count | Counter | target_id:string, method:(get,post,etc), status:(int,error,timeout), status_class:(2xx,3xx,etc,error) |

#### Configuration

- Default retry policy is to retry immediately after error or timeout (so no backoff between retries).
- Default retry strategy is to retry whenever a transport error occurs, or a status code of 5xx is given (no retry on 501 NotImplemented).

Both *Retry Policy* and *Retry Strategy* can be set to use custom implementations, you are not limited by the ones exported by this package.

### Caching

A default caching implementation is provided. It is not recommended to use it outside of the intended purposes of this package.

If only using `EnableCache` option then the returned `Requester` will cache responses on the `httpclient.DefaultCache` store. All
`Requesters` instantiated with this option will share the same store. The store has a default size of 500MB. You can give a custom store
when building the `Requester` by using `WithCache` option; the method `NewLocalCache` allows instantiating an isolated cache for you to use
in your `Requester`.

Please note that because of the way that `NewLocalCache` works, if you build a temporary cache that you want to discard you must call
`Close()` on it or its memory will not be able to be reclamed by the GC.

### Circuit Breaking

The `BucketBreaker` Circuit Breaker implementation provided by the [`breaker`](./pkg/breaker) package is compatible with the interface
expected by `httpclient`.

The `targetID` of the request (as returned by `tracing.TargeID`) is used as a bucket key.

> ⚠️ **IMPORTANT**: Take into consideration that depending on the CircuitBreaker implementation each "bucket" value may allocate background
resources which may not be releasable.
