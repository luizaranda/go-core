package web

import (
	"net/http"
)

// BreakerValidator is a function that determines if a status code written to a
// client by a circuit breaking Handler should count as a success or failure.
// The DefaultBreakerValidator can be used in most situations.
type BreakerValidator func(int) bool

// DefaultBreakerValidator considers any status code less than 500 to be a
// success, from the perspective of a server. All other codes are failures.
func DefaultBreakerValidator(code int) bool { return code < 500 }

type CircuitBreaker interface {
	Allow() bool
	Success()
	Failure()
}

// Breaker produces a Middleware that's governed by the passed Breaker and
// BreakerValidator. Responses written by the next Handler whose status codes
// fail the validator signal failures to the breaker. Once the breaker opens,
// incoming requests are terminated before being answered with HTTP 503.
func Breaker(cb CircuitBreaker, validator BreakerValidator) Middleware {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !cb.Allow() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

			w2 := &responseWriter{w: w, status: http.StatusOK}

			handler(w2, r)

			if validator(w2.Status()) {
				cb.Success()
			} else {
				cb.Failure()
			}
		}
	}
}
