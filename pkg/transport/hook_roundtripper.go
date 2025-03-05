package transport

import (
	"net/http"
)

// HookDecorator returns a RoundTripDecorator that provides hooking capabilities
// to the given http.RoundTripper.
//
// For more information check HookRoundTripper struct.
func HookDecorator(req []RequestHook, res []ResponseHook) RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &HookRoundTripper{
			Transport:    base,
			RequestHook:  req,
			ResponseHook: res,
		}
	}
}

// RequestHook allows a function to run before each request. The HTTP request
// which will be made.
//
// When modifying the request, consider that it is only safe to mutate the
// given request context and headers. All other modifications might
// result in undefined or unwanted behavior.
type RequestHook func(*http.Request) error

// ResponseHook allows running a function after each HTTP response. This function
// will be invoked at the end of every HTTP request executed.
//
// Beware that if the response Body is read and/or closed from
// this method, it will affect the response returned from Do().
type ResponseHook func(*http.Request, *http.Response, error)

// A HookRoundTripper is an implementation of http.RoundTripper that provides a
// way to setup hooks to be called before executing an HTTP Request and after
// the response is received.
type HookRoundTripper struct {
	// The underlying transport actually used to make requests
	Transport http.RoundTripper

	// RequestHook allows a user-supplied function to be called
	// before each request.
	RequestHook []RequestHook

	// ResponseHook allows a user-supplied function to be called
	// with the response from each HTTP request executed.
	ResponseHook []ResponseHook
}

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
//
// It calls the transport RequestHooks before performing each request, and
// the ResponseHook for all responses that did not return an error.
func (t *HookRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Execute all request hooks in the same order they were hooked. In case a hook
	// returns an error then we stop the request flow and return it to the caller.
	for _, hook := range t.RequestHook {
		if err := hook(req); err != nil {
			return nil, err
		}
	}

	res, err := t.Transport.RoundTrip(req)

	// We only execute response hooks if there was no error as we can't assert
	// the state of the http.Response object on a RoundTrip error.
	for _, hook := range t.ResponseHook {
		hook(req, res, err)
	}

	return res, err
}
