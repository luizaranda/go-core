package httpclient

import (
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luizaranda/go-core/pkg/telemetry"
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
	"github.com/luizaranda/go-core/pkg/transport"
)

var (
	_defaultTransport = transport.NewPooled("core-default")
)

// DefaultTransport returns the default transport used by New and NewRetryable
// if none is given.
//
// It may be used freely outside this package.
func DefaultTransport() *transport.PooledTransport {
	return _defaultTransport
}

// Requester exposes the http.Client.Do method, which is the minimum
// required method for executing HTTP requests.
type Requester interface {
	Do(*http.Request) (*http.Response, error)
}

type clientOptions struct {
	Timeout           time.Duration
	CheckRedirect     CheckRedirectFunc
	Transport         *transport.PooledTransport
	ReqHooks          []transport.RequestHook
	ResHooks          []transport.ResponseHook
	Cache             transport.Cache
	CircuitBreaker    transport.CircuitBreaker
	EnableClientTrace bool
}

type retryOptions struct {
	clientOptions
	BackoffStrategy BackoffFunc
	CheckRetry      CheckRetryFunc
}

// Option signature for client configurable parameters.
type Option interface {
	OptionRetryable
	applyClient(opts *clientOptions)
}

// OptionRetryable signature for retryable client configurable parameters.
type OptionRetryable interface {
	applyRetryable(opts *retryOptions)
}

type optFunc func(opts *clientOptions)

func (f optFunc) applyClient(o *clientOptions)   { f(o) }
func (f optFunc) applyRetryable(o *retryOptions) { f(&o.clientOptions) }

type retryableOptFunc func(opts *retryOptions)

func (f retryableOptFunc) applyRetryable(o *retryOptions) { f(o) }

// WithTransport controls the base HTTP transport to use for executing the HTTP
// requests.
//
// We force the usage of a PooledTransport so that application can track
// connection pools. You can easily transform an *http.Transport into a
// *transport.PooledTransport by using transport.NewPooledFromTransport.
func WithTransport(t *transport.PooledTransport) Option {
	return optFunc(func(options *clientOptions) {
		options.Transport = t
	})
}

// DisableTimeout disables the timeout for outgoing requests.
//
// Requests may still timeout if Requester needs to establish a new TCP conn as
// underlying http.Transport timeout will still be in effect.
func DisableTimeout() Option { return WithTimeout(0) }

// WithTimeout controls the timeout for each request. When retrying requests,
// each retried request will start counting from the beginning towards this
// timeout.
//
// Requests may still timeout earlier if Requester needs to establish a new TCP
// conn as underlying http.Transport timeout will be taken into account.
//
// A timeout of 0 disables request timeouts.
func WithTimeout(t time.Duration) Option {
	return optFunc(func(options *clientOptions) {
		// Negative durations do not make sense in the context of an Requester.
		if t >= 0 {
			options.Timeout = t
		}
	})
}

// FollowRedirects controls whether the client should follow HTTP redirects.
// The default policy is to not follow redirects. In case follow=true is
// given, then a max of 10 redirects will be followed.
func FollowRedirects(follow bool) Option {
	return optFunc(func(options *clientOptions) {
		if follow {
			options.CheckRedirect = http.Client{}.CheckRedirect
		} else {
			options.CheckRedirect = NoRedirect
		}
	})
}

// WithRequestHook allows the user to add additional request hooks to be
// executed during an HTTP request.
func WithRequestHook(hooks ...transport.RequestHook) Option {
	return optFunc(func(options *clientOptions) {
		options.ReqHooks = append(options.ReqHooks, hooks...)
	})
}

// WithResponseHook allows the user to add additional response hooks to be
// executed during an HTTP response.
func WithResponseHook(hooks ...transport.ResponseHook) Option {
	return optFunc(func(options *clientOptions) {
		options.ResHooks = append(options.ResHooks, hooks...)
	})
}

// EnableCache enables HTTP caching for the given httpclient. It uses the global
// DefaultCache as the backing store.
//
// Cache storage can be customized by using WithCache option. If EnableCache is
// called after WithCache then it doesn't overwrite the storage.
func EnableCache() Option {
	return optFunc(func(options *clientOptions) {
		// Only set it to DefaultCache if it's not already set. This allows
		// calling EnableCache after using WithCache for giving a custom one.
		if options.Cache == nil {
			options.Cache = DefaultCache
		}
	})
}

// WithCache allows the user to set the storage used for caching HTTP responses.
//
// If given nil then caching is disabled.
func WithCache(cache transport.Cache) Option {
	return optFunc(func(options *clientOptions) {
		options.Cache = cache
	})
}

// WithCircuitBreaker allows the user to set the circuit breaker to use in the
// httpclient. Requests will be bucketed in the circuit breaker based on their
// `tracing.EndpointTemplate` value.
func WithCircuitBreaker(cb transport.CircuitBreaker) Option {
	return optFunc(func(options *clientOptions) {
		options.CircuitBreaker = cb
	})
}

// WithEnableClientTrace enables the tracing of low level metrics
// of the HTTP requests performed by the httpclient.
func WithEnableClientTrace() Option {
	return optFunc(func(options *clientOptions) {
		options.EnableClientTrace = true
	})
}

// WithBackoffStrategy controls the wait time between requests when retrying.
func WithBackoffStrategy(strategy BackoffFunc) OptionRetryable {
	return retryableOptFunc(func(options *retryOptions) {
		options.BackoffStrategy = strategy
	})
}

// WithRetryPolicy controls the retry policy of the given HTTP client.
func WithRetryPolicy(checkRetry CheckRetryFunc) OptionRetryable {
	return retryableOptFunc(func(options *retryOptions) {
		options.CheckRetry = checkRetry
	})
}

var (
	// DefaultTimeout is the timeout used by default when building a Client.
	DefaultTimeout = 3 * time.Second

	// DefaultBackoffStrategy is the retry strategy used by default when
	// building a Client.
	DefaultBackoffStrategy = ConstantBackoff(0)

	// DefaultCheckRedirect is the redirect strategy used by default when
	// building a Client.
	// Default is to not follow HTTP redirects.
	DefaultCheckRedirect = CheckRedirectFunc(NoRedirect)

	// DefaultRetryPolicy is the function that tells on any given request if the
	// client should retry it or not. By default, it retries on connection and 5xx errors only.
	DefaultRetryPolicy = ServerErrorsRetryPolicy()
)

// New builds a *http.Client which keeps TCP connections to destination servers
// and records telemetry on all executed requests.
//
// Returned client can be customized by passing options to New.
func New(opts ...Option) *http.Client {
	config := clientOptions{
		Timeout:       DefaultTimeout,
		CheckRedirect: DefaultCheckRedirect,
		ReqHooks:      []transport.RequestHook{ForwardTracingHeadersRequestHook},
		Transport:     DefaultTransport(),
	}

	for _, opt := range opts {
		opt.applyClient(&config)
	}

	return &http.Client{
		Timeout:       config.Timeout,
		CheckRedirect: config.CheckRedirect,
		Transport:     roundTripper(&config),
	}
}

// NewRetryable builds a *RetryableClient which keeps TCP connections to
// destination servers, records telemetry on all executed requests, and can
// retry requests on error.
//
// RetryableClient can be customized by passing options to it. Note that Option
// is of type OptionRetryable, so those functional options can be used as well.
//
// RetryMax tells the client the maximum number of retries to execute. Eg.: A
// value of 3, means to execute the original request, and up-to 3 retries (4
// requests in total). A value of 0 means no retries, essentially the same as
// building a *http.Client with New.
func NewRetryable(retryMax int, opts ...OptionRetryable) *RetryableClient {
	config := retryOptions{
		BackoffStrategy: DefaultBackoffStrategy,
		CheckRetry:      DefaultRetryPolicy,
		clientOptions: clientOptions{
			Timeout:       DefaultTimeout,
			CheckRedirect: DefaultCheckRedirect,
			ReqHooks:      []transport.RequestHook{ForwardTracingHeadersRequestHook, RetryHeaderHook},
			ResHooks:      []transport.ResponseHook{RetryMetricHook},
			Transport:     DefaultTransport(),
		},
	}

	for _, opt := range opts {
		opt.applyRetryable(&config)
	}

	return &RetryableClient{
		RetryMax:        retryMax,
		BackoffStrategy: config.BackoffStrategy,
		CheckRetry:      config.CheckRetry,
		Client: &http.Client{
			Timeout:       config.Timeout,
			CheckRedirect: config.CheckRedirect,
			Transport:     roundTripper(&config.clientOptions),
		},
	}
}

func roundTripper(config *clientOptions) http.RoundTripper {
	chain := transport.RoundTripChain{transport.UserAgentDecorator()}

	if config.Cache != nil {
		chain = append(chain, transport.CacheDecorator(config.Cache))
	}

	chain = append(chain, transport.HookDecorator(config.ReqHooks, config.ResHooks))

	if config.EnableClientTrace {
		chain = append(chain, transport.ExtendedTraceDecorator())
	} else {
		chain = append(chain, transport.TraceDecorator())
	}

	if config.CircuitBreaker != nil {
		chain = append(chain, transport.CircuitBreakerDecorator(
			config.CircuitBreaker,
			transport.DefaultCircuitBreakerCheckFunc(),
			// Use the TargetID or EndpointTemplate as the circuit breaker bucket key.
			func(r *http.Request) string {
				targetID := tracing.TargetID(r.Context())
				if targetID == "" {
					return tracing.EndpointTemplate(r.Context())
				}
				return targetID
			},
		))
	}

	// OpenTelemetryDecorator must be last to avoid conflict with the TraceDecorator
	chain = append(chain, transport.OpenTelemetryDecorator())

	return chain.Apply(config.Transport)
}

// ForwardTracingHeadersRequestHook adds to the outgoing request any headers
// contained in the context that should be forwarded for tracing reasons.
//
// The forwarded headers that already exist in the request will preserve their values
// even if they are empty. The function records a metric when a different forwarded
// header value exists in the context.
func ForwardTracingHeadersRequestHook(req *http.Request) error {
	for header, value := range tracing.ForwardedHeaders(req.Context()) {
		// If the header was already added by the caller, and it's different from the
		// one we should be forwarding, instead of replacing it record a metric.
		if v, ok := req.Header[textproto.CanonicalMIMEHeaderKey(header)]; ok && len(v) > 0 && v[0] != value {
			telemetry.Incr(req.Context(), "platform.traffic.forwarded_header.diff", telemetry.Tags(
				"stack", "go",
				"header", strings.ToLower(header),
				"target_id", telemetry.SanitizeMetricTagValue(tracing.TargetID(req.Context())),
			))
			continue
		}

		req.Header.Set(header, value)
	}
	return nil
}

// RetryHeaderHook adds the x-retry header to the outgoing request if the
// request is a retry. The value of the header is the retry attempt number.
func RetryHeaderHook(req *http.Request) error {
	if i := RetryCount(req); i > 0 {
		req.Header.Set("x-retry", strconv.Itoa(i))
	}
	return nil
}

// RetryMetricHook is a response hook which records a metric with request
// information when the response corresponds to a request which was a retry.
func RetryMetricHook(req *http.Request, res *http.Response, err error) {
	if i := RetryCount(req); i == 0 {
		return
	}

	status, statusClass := "error", "error"
	if err == nil {
		status = strconv.Itoa(res.StatusCode)
		statusClass = strconv.Itoa(res.StatusCode/100) + "xx" // 2xx, 3xx, 4xx, 5xx, etc
	} else if os.IsTimeout(err) {
		status = "timeout"
	}

	tags := []string{
		"technology:go",
		"target_id:" + telemetry.SanitizeMetricTagValue(tracing.TargetID(req.Context())),
		"method:" + strings.ToLower(req.Method),
		"status:" + status,
		"status_class:" + statusClass,
	}

	telemetry.Incr(req.Context(), "toolkit.http.client.request.retry.count", tags)
}
