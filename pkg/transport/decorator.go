package transport

import (
	"net/http"
)

// RoundTripDecorator is a named type for any function that takes a RoundTripper
// and returns a RoundTripper.
type RoundTripDecorator func(http.RoundTripper) http.RoundTripper

// RoundTripChain is an ordered collection of RoundTripDecorator.
type RoundTripChain []RoundTripDecorator

// Apply wraps the given RoundTripper with the RoundTripDecorator chain.
func (c RoundTripChain) Apply(base http.RoundTripper) http.RoundTripper {
	for x := len(c) - 1; x >= 0; x = x - 1 {
		base = c[x](base)
	}
	return base
}
