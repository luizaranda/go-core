package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/newrelic/go-agent/v3/newrelic"
)

var (
	_defaultBufferLen = 500
	_defaultTimeout   = 200 * time.Millisecond
	_defaultRate      = 1.0
	_shutdownTimeout  = 5 * time.Second

	// By default, when using NR http.ResponseWriter(as we do), response codes that are
	// greater than or equal to 400 or less than 100 -- with the exception
	// of 0 (gRPC OK), 5 (gRPC NOT_FOUND), and 404 -- are turned into errors.
	// Additionally, we can specify a list of status code to ignore.
	//
	// Since our framework provides users the ability to control what and when to notify an error
	// by using the web.HandlerError and a default error handler (or a custom that may be provided),
	// we don't want to either duplicate an error nor throw an unexpected one.
	// For that, we ignore any 4xx or 5xx status code that is written by the http.ResponseWriter.
	_errorCollectorIgnoredStatusCodes = []int{
		// 4xx status code
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusNotAcceptable,
		http.StatusProxyAuthRequired,
		http.StatusRequestTimeout,
		http.StatusConflict,
		http.StatusGone,
		http.StatusLengthRequired,
		http.StatusPreconditionFailed,
		http.StatusRequestEntityTooLarge,
		http.StatusRequestURITooLong,
		http.StatusUnsupportedMediaType,
		http.StatusRequestedRangeNotSatisfiable,
		http.StatusExpectationFailed,
		http.StatusTeapot,
		http.StatusMisdirectedRequest,
		http.StatusUnprocessableEntity,
		http.StatusLocked,
		http.StatusFailedDependency,
		http.StatusTooEarly,
		http.StatusUpgradeRequired,
		http.StatusPreconditionRequired,
		http.StatusTooManyRequests,
		http.StatusRequestHeaderFieldsTooLarge,
		http.StatusUnavailableForLegalReasons,
		// Custom http status codes to ignore
		499, /*client closed request*/
		// 5xx status code
		http.StatusInternalServerError,
		http.StatusNotImplemented,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusHTTPVersionNotSupported,
		http.StatusVariantAlsoNegotiates,
		http.StatusInsufficientStorage,
		http.StatusLoopDetected,
		http.StatusNotExtended,
		http.StatusNetworkAuthenticationRequired,
	}
)

// DefaultTracer is the default tracer and is used when calling a function on
// telemetry package with a context with no associated telemetry.Client instance.
//
// DefaultTracer by default discards all metrics. You can change it's
// implementation by settings this variable to an instantiated tracer.
var DefaultTracer = NewNoOpClient()

// A client is a handle for performing telemetry operations. It is safe to
// use one client from multiple goroutines simultaneously.
type client struct {
	nrApp  *newrelic.Application
	statsd statsd.ClientInterface
}

var _ Client = (*client)(nil)

// Config contains attributes required by NewClient to bootstrap itself.
type Config struct {
	// ApplicationName is the name that will be shown on NewRelic.
	ApplicationName string

	// NewRelicLicense is the license used for connecting with NewRelic. This
	// license identifies the account to use.
	NewRelicLicense string

	// NewRelicHighSecurity guarantees that certain agent settings can not be
	// made more permissive. This setting must match the corresponding account
	// setting in the New Relic UI.
	NewRelicHighSecurity bool

	// NewRelicApplication lets the client inject an already instantiated
	// NewRelic application. In this case all other NewRelic config values are
	// ignored.
	//
	// Deprecated: Available for legacy integration. Avoid usage.
	NewRelicApplication *newrelic.Application

	// DatadogAddress is the address of the datadog agent to which statsd must
	// connect to.
	DatadogAddress string
}

// NewClient returns a new client connected to all tracing providers.
func NewClient(cfg Config) (Client, error) {
	nrApp := cfg.NewRelicApplication
	if nrApp == nil {
		nrOpts := []newrelic.ConfigOption{
			newrelic.ConfigEnabled(true),
			newrelic.ConfigLicense(cfg.NewRelicLicense),
			newrelic.ConfigAppName(cfg.ApplicationName),
			newrelic.ConfigDistributedTracerEnabled(false),
			newrelic.ConfigFromEnvironment(),
			func(config *newrelic.Config) {
				config.ErrorCollector.IgnoreStatusCodes = _errorCollectorIgnoredStatusCodes
				config.HighSecurity = cfg.NewRelicHighSecurity
			},
		}

		app, err := newrelic.NewApplication(nrOpts...)
		if err != nil {
			return nil, err
		}
		nrApp = app
	}

	opts := []statsd.Option{
		statsd.WithMaxMessagesPerPayload(_defaultBufferLen),
		statsd.WithWriteTimeout(_defaultTimeout),
	}

	s, err := statsd.New(cfg.DatadogAddress, opts...)
	if err != nil {
		return nil, err
	}

	return &client{
		nrApp:  nrApp,
		statsd: s,
	}, nil
}

// NewNoOpClient is a telemetry client that does nothing. Can be useful in testing
// situations for library users.
func NewNoOpClient() Client {
	nrApp, _ := newrelic.NewApplication(newrelic.ConfigEnabled(false))
	return &client{
		statsd: &statsd.NoOpClient{},
		nrApp:  nrApp,
	}
}

// Close closes the telemetry client, flushing all metrics contained in buffers.
func (c *client) Close() error {
	c.nrApp.Shutdown(_shutdownTimeout)
	return c.statsd.Close()
}

// StartSpan begins a Span.
// - This method never returns nil.
// - Caller must call Finish on the returned Span for recording to occur.
func (c *client) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return c.StartWebSpan(ctx, name, nil, nil)
}

// StartWebSpan starts a Span.
//   - This method never returns nil.
//   - Caller must call Finish on the returned Span for recording to occur.
//   - The returned Span may implement http.ResponseWriter interface. This depends
//     on the provider.
func (c *client) StartWebSpan(ctx context.Context, name string, w http.ResponseWriter,
	r *http.Request) (context.Context, Span) {

	tx := newrelic.FromContext(ctx)
	if tx != nil {
		return StartSpan(ctx, name)
	}

	nrTx := c.nrApp.StartTransaction(name)

	// It is not required for the caller to give us both the *http.Request that
	// initiated the transaction as well as the http.ResponseWriter that will be
	// used for responding. Do not couple SetWebRequestHTTP method with having a
	// non-nil ResponseWriter given.
	if r != nil {
		nrTx.SetWebRequestHTTP(r)
	}

	// Build the Span to be returned. In case we have a http.ResponseWriter the
	// transaction SetWebResponse method returns a new wrapped one that allows
	// automatic propagation of values to NewRelic's transaction. When this
	// happens we want to return a different implementation of a Span.
	var span Span
	if w == nil {
		span = &nrTransactionSpan{Transaction: nrTx}
	} else {
		rw := nrTx.SetWebResponse(w)
		span = &nrWebTransactionSpan{
			ResponseWriter:    rw,
			nrTransactionSpan: &nrTransactionSpan{Transaction: nrTx},
		}
	}

	return contextWithTransaction(ctx, nrTx, c), span
}

// Gauge measures the value of a metric at a particular time.
func (c *client) Gauge(name string, value float64, tags []string) {
	_ = c.statsd.Gauge(name, value, tags, _defaultRate)
}

// Count tracks how many times something happened per second.
func (c *client) Count(name string, value int64, tags []string) {
	_ = c.statsd.Count(name, value, tags, _defaultRate)
}

// Incr is just Count of 1.
func (c *client) Incr(name string, tags []string) {
	_ = c.statsd.Incr(name, tags, _defaultRate)
}

// Decr is just Count of -1.
func (c *client) Decr(name string, tags []string) {
	_ = c.statsd.Decr(name, tags, _defaultRate)
}

// Histogram tracks the statistical distribution of a set of values on each host.
func (c *client) Histogram(name string, value float64, tags []string) {
	_ = c.statsd.Histogram(name, value, tags, _defaultRate)
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func (c *client) Distribution(name string, value float64, tags []string) {
	_ = c.statsd.Distribution(name, value, tags, _defaultRate)
}

// Set counts the number of unique elements in a group.
func (c *client) Set(name string, value string, tags []string) {
	_ = c.statsd.Set(name, value, tags, _defaultRate)
}

// Timing sends timing information, it is an alias for TimeInMilliseconds.
func (c *client) Timing(name string, value time.Duration, tags []string) {
	_ = c.statsd.Timing(name, value, tags, _defaultRate)
}

// TimeInMilliseconds sends timing information in milliseconds.
// It is flushed by statsd with percentiles, mean and other info
// (https://github.com/etsy/statsd/blob/master/docs/metric_types.md#timing).
func (c *client) TimeInMilliseconds(name string, value float64, tags []string) {
	_ = c.statsd.TimeInMilliseconds(name, value, tags, _defaultRate)
}
