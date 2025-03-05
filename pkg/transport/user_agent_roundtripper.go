package transport

import (
	"net/http"

	"github.com/luizaranda/go-core/pkg/internal"
)

// UserAgentDecorator returns a RoundTripDecorator that sets a default User-Agent to the http.Request.
func UserAgentDecorator() RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &UserAgentRoundTripper{Transport: base}
	}
}

// UserAgentRoundTripper is a http.RoundTripper that sets a default User-Agent header only if the client
// does not provide one.
// The format is httpclient-go/x.y.z where x.y.z is the current go-core's build version.
type UserAgentRoundTripper struct {
	Transport http.RoundTripper
}

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
func (ua *UserAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.UserAgent() == "" {
		req.Header.Set("User-Agent", "httpclient-go/"+internal.Version)
	}

	return ua.Transport.RoundTrip(req)
}
