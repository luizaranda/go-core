package transport

import (
	"net/http"

	"github.com/luizaranda/go-core/pkg/transport/internal/httpcache"
)

// A Cache interface is used by the Transport to store and retrieve responses.
type Cache interface {
	// Get returns the []byte representation of a cached response and a bool set
	// to true if the value isn't empty
	Get(key string) (responseBytes []byte, ok bool)
	// Set stores the []byte representation of a response against a key
	Set(key string, responseBytes []byte)
	// Delete removes the value associated with the key
	Delete(key string)
}

// CacheDecorator returns a RoundTripDecorator that provides caching
// capabilities to the given http.RoundTripper by wrapping RoundTrip calls to
// return from a cache where possible (avoiding the HTTP request). It will
// additionally add validators (etag/if-modified-since) to repeated requests
// allowing servers to return 304 / Not Modified.
func CacheDecorator(cache Cache) RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &httpcache.Transport{
			Transport:           base,
			Cache:               cache,
			MarkCachedResponses: true,
		}
	}
}
