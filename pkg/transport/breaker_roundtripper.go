package transport

import (
	"errors"
	"net/http"

	"github.com/luizaranda/go-core/pkg/telemetry"
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
)

type CircuitBreaker interface {
	Allow(bucket string) (allowed bool, success, failure func())
}

// CircuitBreakerCheckFunc callback indicates the transport how it should map
// an http.Response into a CircuitBreaker Success or Failure call.
// Success is called if true is returned, Failure if false.
type CircuitBreakerCheckFunc func(*http.Response) bool

// DefaultCircuitBreakerCheckFunc returns a CircuitBreakerCheckFunc which
// signals the breaker a failure occurred on any 5xx response status code.
func DefaultCircuitBreakerCheckFunc() CircuitBreakerCheckFunc {
	return func(r *http.Response) bool {
		return r.StatusCode < 500
	}
}

// CircuitBreakerDecorator returns a CircuitBreakerRoundTripper that provides
// circuit breaking capabilities to the given http.RoundTripper.
//
// For more information check CircuitBreakerRoundTripper struct.
func CircuitBreakerDecorator(cb CircuitBreaker, f CircuitBreakerCheckFunc, b func(r *http.Request) string) RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &CircuitBreakerRoundTripper{
			Base:           base,
			CircuitBreaker: cb,
			CheckFunc:      f,
			BucketFunc:     b,
		}
	}
}

// CircuitBreakerRoundTripper simplifies the process of using a circuit breaker
// at the transport level. Each request is mapped into a bucket which is then
// passed into the Circuit Breaker to see if it's allowed or not.
type CircuitBreakerRoundTripper struct {
	Base           http.RoundTripper
	CircuitBreaker CircuitBreaker

	// CheckFunc lets the round tripper signal the circuit breaker that a
	// request that returned a response is indeed an error.
	//
	// Error from the underlying RoundTripper are automatically signaled as such
	// to the circuit breaker.
	CheckFunc CircuitBreakerCheckFunc

	// BucketFunc returns the Circuit Breaker bucket into which the request
	// being round-tripped belongs to.
	BucketFunc func(r *http.Request) string
}

func (b *CircuitBreakerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	bucket := b.BucketFunc(r)

	allowed, success, failure := b.CircuitBreaker.Allow(bucket)
	if !allowed {
		commonTags := breakerCommonTags(r)
		telemetry.Incr(r.Context(), "toolkit.http.client.circuit_breaker.open", commonTags)
		return nil, ErrCircuitOpen
	}

	res, err := b.Base.RoundTrip(r)
	if err != nil {
		failure()
		return res, err
	}

	switch b.CheckFunc(res) {
	case true:
		success()
	case false:
		failure()
	}

	return res, nil
}

func breakerCommonTags(req *http.Request) []string {
	targetID := tracing.TargetID(req.Context())

	if targetID == "" {
		return []string{}
	}

	return telemetry.Tags(
		"target_id", targetID,
		"bucket", targetID,
	)
}

// ErrCircuitOpen is raise when the circuit breaker is open.
var ErrCircuitOpen = errors.New("transport: circuit breaker open")
