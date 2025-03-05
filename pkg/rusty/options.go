package rusty

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type commonOptions struct {
	Header   http.Header
	TargetID string
}

type requestOptions struct {
	commonOptions
	Params      map[string]string
	Query       url.Values
	RequestBody any
}

type endpointOptions struct {
	commonOptions
	ErrorPolicyFn ErrorPolicyFunc
}

// Option interface is implemented by option functions that are available both at endpoint creation and request invocations.
type Option interface {
	EndpointOption
	RequestOption
	apply(opt *commonOptions)
}

// EndpointOption interface is implemented by option functions that are only available when creating an endpoint.
type EndpointOption interface {
	applyEndpoint(op *endpointOptions)
}

// RequestOption interface is implemented by option functions that are only available for request invocations.
type RequestOption interface {
	applyRequest(opt *requestOptions)
}

type allOptionFunc func(opt *commonOptions)

func (f allOptionFunc) apply(o *commonOptions)           { f(o) }
func (f allOptionFunc) applyRequest(o *requestOptions)   { f(&o.commonOptions) }
func (f allOptionFunc) applyEndpoint(o *endpointOptions) { f(&o.commonOptions) }

type endpointOptionFunc func(opt *endpointOptions)

func (f endpointOptionFunc) applyEndpoint(o *endpointOptions) { f(o) }

type requestOptionFunc func(opt *requestOptions)

func (f requestOptionFunc) applyRequest(o *requestOptions) { f(o) }

// WithParam will set value into the name placeholder either in the path and/or the query string of the endpoint URI.
// The value type can be string, the integer types or Stringer, any other type will panic.
func WithParam(name string, value any) RequestOption {
	return requestOptionFunc(func(options *requestOptions) {
		options.Params[name] = toString(value)
	})
}

// WithHeader will set a header with a value.
// The value type can be string, the integer types or Stringer, any other type will panic.
func WithHeader(name string, value any) Option {
	return allOptionFunc(func(options *commonOptions) {
		options.Header.Add(name, toString(value))
	})
}

// WithParamObject will map every field value of struct into corresponding placeholders.
// Placeholder name will be inferred from field name, if exported.
// You can override this behavior by using the field tag `param:"placeholder_name"`.
// If you want a particular field to be ignored you can use `param:"-"`.
// The value type can be string, the integer types or Stringer, any other type will panic.
// If object is nil or not a struct (or a pointer to a struct) then it will panic.
func WithParamObject(object any) RequestOption {
	return requestOptionFunc(func(options *requestOptions) {
		options.Params = getParams(object)
	})
}

// WithBody will set a body to the request. Can be a []byte, an io.Reader or any other type
// that can be marshaled to JSON. If it's the latter you must provide a
// Content-Type header to let rusty know how to encode it. If you don't then an
// ErrUnsupportedBodyType will be returned in any of the Request functions (Post, Put, etc).
func WithBody(body any) RequestOption {
	return requestOptionFunc(func(options *requestOptions) {
		options.RequestBody = body
	})
}

// WithErrorPolicy control whether a response in a request should be treated as an error or not in your application.
// Default is treat all transport errors and any response status >=400 as an error.
func WithErrorPolicy(fn ErrorPolicyFunc) EndpointOption {
	return endpointOptionFunc(func(options *endpointOptions) {
		options.ErrorPolicyFn = fn
	})
}

// WithTarget sets the telemetry targetID to use in requests to this endpoint.
// Deprecated: use WithTargetID instead.
func WithTarget(targetID string) Option {
	return allOptionFunc(func(options *commonOptions) {
		options.TargetID = targetID
	})
}

// WithTargetID sets the telemetry target id attribute to use in all
// the requests made to this endpoint.
// It should have the lowest cardinality possible.
// For example, a good target id would be /api/v1/users/{user_id}
// or /api/v1/users/{user_id}/orders/{order_id}.
// Take into account that this value is also used as the bucket name when using
// the breaker.BucketBreaker implementation.
func WithTargetID(targetID string) Option {
	return allOptionFunc(func(options *commonOptions) {
		options.TargetID = targetID
	})
}

// WithQuery adds additional query values than those specified and parameterized in the endpointURL.
// If a query parameter is both in endpointURL at creation and in the url.Values map received as
// parameter the latter is also appended at the end.
func WithQuery(v url.Values) RequestOption {
	return requestOptionFunc(func(options *requestOptions) {
		options.Query = v
	})
}

func toString(value any) string {
	switch t := value.(type) {
	case string:
		return t
	case time.Time:
		return t.Format(time.RFC3339)
	case bool:
		return strconv.FormatBool(t)
	case fmt.Stringer:
		return t.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", value)
	default:
		panic(fmt.Sprintf("type %T is unsupported", value))
	}
}

func defaultEndpointOptions() endpointOptions {
	return endpointOptions{
		commonOptions: defaultOptions(),
		ErrorPolicyFn: DefaultErrorPolicy,
	}
}

func defaultRequestOptions() requestOptions {
	return requestOptions{
		commonOptions: defaultOptions(),
		Params:        make(map[string]string),
	}
}

func defaultOptions() commonOptions {
	return commonOptions{
		Header: make(http.Header),
	}
}
