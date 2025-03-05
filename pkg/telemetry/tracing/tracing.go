package tracing

import (
	"context"
	"encoding/json"
	"net/textproto"
	"strings"

	"github.com/gofrs/uuid"
)

// tracingKey type is an internal type used for
// assigning values to context.Context in a way that
// only this package is able to access.
// Example:
//
//	ctx := context.WithValue(context.Background(), tracingKey, "value")
//
//	ctx.Value(rqCtxKey) // Read previous saved value from context
type tracingKey struct{}

const (
	// RequestIDHeader exposes the Header to use for reading
	// and propagating the request id from an transport context.
	RequestIDHeader = "x-request-id"

	// RequestFlowStarterHeader is the transport Header that the tracing
	// library forwards when the application start a new request flow.
	RequestFlowStarterHeader = "x-flow-starter"

	// ForwardedHeadersName is the transport Header that contains the comma
	// separated value of request headers that must be forwarded to the
	// outgoing HTTP request that the application performs.
	ForwardedHeadersName = "x-forwarded-header-names"
)

// TraceableGetSetter interface is the interface a "bag" of headers needs to
// comply so that calling ContextFromHeader can decorate the given context with
// tracing information.
//
// This interface is provided instead of requiring the Header struct as to allow
// other implementations of transport specific headers (like http.Header) to
// be used, which simplifies the usage of ContextFromHeader to the user.
type TraceableGetSetter interface {
	Get(string) string
	Set(string, string)
}

// Header type contains key value pairs of information we consider traceable.
//
// User is expected to retrieve the Header struct from a context using the
// ForwardedHeaders function and add them as metadata to the transport of choice.
type Header map[string]string

func (h Header) Set(key string, val string) { h[norm(key)] = val }
func (h Header) Get(key string) string      { return h[norm(key)] }
func norm(s string) string                  { return textproto.CanonicalMIMEHeaderKey(s) }

func (h Header) UnmarshalJSON(b []byte) error {
	tmp := make(map[string]string)
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	for k := range tmp {
		// Call Set so that we guarantee we normalize
		// all keys of the input JSON.
		h.Set(k, tmp[k])
	}

	return nil
}

// ContextFromHeader returns a context decorated with the headers from the
// transport request that must be forwarded by the application in requests to
// external services, independently on the transport. The headers with empty
// values are not forwarded.
func ContextFromHeader(ctx context.Context, h TraceableGetSetter) context.Context {
	return contextFromHeader(ctx, h, false)
}

// ContextFromHTTPHeader returns a context decorated with the headers from the
// transport request that must be forwarded by the application in requests to
// external services, independently on the transport. The headers are forwarded
// despite they have empty values.
// This function must be used to avoid traceability compliance issues.
func ContextFromHTTPHeader(ctx context.Context, h TraceableGetSetter) context.Context {
	return contextFromHeader(ctx, h, true)
}

func contextFromHeader(ctx context.Context, h TraceableGetSetter, includeEmpty bool) context.Context {
	headers := make(Header)

	// Read the Header that lists all headers to be forwarded and add
	// them to the headers map.
	forwardedHeadersName := h.Get(ForwardedHeadersName)
	if forwardedHeadersName != "" {
		forwardedHeaders := strings.Split(forwardedHeadersName, ",")
		for _, header := range forwardedHeaders {
			key := strings.TrimSpace(header)
			if value := h.Get(key); value != "" || includeEmpty {
				headers.Set(key, value)
			}
		}
	}

	// Check to see if x-request-id is forwarded from the request. If not
	// generate a new request id and assign to the Header h.
	headers.Set(RequestIDHeader, h.Get(RequestIDHeader))
	if reqID := headers.Get(RequestIDHeader); reqID == "" {
		headers.Set(RequestIDHeader, newRequestID())
	}

	return context.WithValue(ctx, tracingKey{}, headers)
}

// ForwardedHeaders returns the headers that must be forwarded by a transport
// client given a request context.Context.
func ForwardedHeaders(ctx context.Context) Header {
	headers, ok := ctx.Value(tracingKey{}).(Header)
	if !ok {
		return make(Header)
	}
	return headers
}

// RequestID returns the request id given a context.
// If the context does not contain a requestID, then
// an empty string is returned.
func RequestID(ctx context.Context) string {
	headers := ForwardedHeaders(ctx)
	return headers.Get(RequestIDHeader)
}

// NewFlowStarterContext decorates the given context with a
// request id and marks it as an internal request.
func NewFlowStarterContext(ctx context.Context) context.Context {
	headers := make(Header)

	headers.Set(RequestIDHeader, newRequestID())
	headers.Set(RequestFlowStarterHeader, "true")

	return context.WithValue(ctx, tracingKey{}, headers)
}

// newRequestID generates a new UUIDv4. If generation fails, which
// could happen if the randomness source is depleted, then it
// returns an empty string.
func newRequestID() string {
	u, err := uuid.NewV4()
	if err != nil {
		return ""
	}
	return u.String()
}
