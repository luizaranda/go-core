package rusty

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/luizaranda/go-core/pkg/internal"
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
	"github.com/luizaranda/go-core/pkg/transport/httpclient"
)

var (
	// ErrUnsupportedBodyType body type passed to http client is unsupported.
	ErrUnsupportedBodyType = errors.New("unsupported body type")

	// ErrEmptyURLParam empty param value for replacing in a rusty URL.
	ErrEmptyURLParam = errors.New("empty param value for a rusty URL")

	// ErrMissingURLParam missing param for replacing in a rusty URL.
	ErrMissingURLParam = errors.New("missing param value for a rusty URL")
)

// Requester is responsible for making HTTP requests. It is usually an implementation provided
// by the transport package (e.g. httpclient).
// It can also be a http.Client or even a mock implementation for testing.
type Requester interface {
	// Do makes an HTTP request and returns an HTTP response.
	Do(*http.Request) (*http.Response, error)
}

// Response represents the response from an Endpoint call that succeeded.
type Response struct {
	// Body is the complete response body.
	Body []byte
	// StatusCode is the response status code.
	StatusCode int
	// Header is the response header map.
	Header http.Header
}

// Endpoint represents an API endpoint at a particular URL. It is safe to use concurrently by multiple goroutines.
// It is expected to be created once and shared across the lifetime of the application.
type Endpoint struct {
	requester      Requester
	formatURL      *url.URL
	defaultHeaders http.Header
	errorPolicy    ErrorPolicyFunc
	targetID       string
}

// ErrorPolicyFunc for specifying an error policy function that will be used to determine if an error should be returned.
// It is called only if the HTTP request was successful regardless of the status code.
// It won't be called if the HTTP request failed due to a network error or a timeout.
// If that's the case, the error returned by the HTTP client will be returned.
type ErrorPolicyFunc func(*Response) error

// DefaultErrorPolicy policy will return an error when the status code of the response is greater than 399.
var DefaultErrorPolicy ErrorPolicyFunc = func(r *Response) error {
	if r.StatusCode < 400 {
		return nil
	}

	return &Error{r}
}

// NewEndpoint creates a new Endpoint with the given URL and options.
// It returns an error if endpointURL is not a valid URL as defined by url.ParseRequestURI.
func NewEndpoint(requester Requester, endpointURL string, opts ...EndpointOption) (*Endpoint, error) {
	options := defaultEndpointOptions()
	for _, option := range opts {
		option.applyEndpoint(&options)
	}

	u, err := url.ParseRequestURI(endpointURL)
	if err != nil {
		return nil, err
	}

	return &Endpoint{
		requester:      requester,
		formatURL:      u,
		defaultHeaders: options.Header,
		errorPolicy:    options.ErrorPolicyFn,
		targetID:       options.TargetID,
	}, nil
}

// Get will issue a http get request to the endpoint.
func (e *Endpoint) Get(ctx context.Context, optionFns ...RequestOption) (*Response, error) {
	return e.doRequest(ctx, http.MethodGet, optionFns...)
}

// Post will issue a post request to the endpoint.
func (e *Endpoint) Post(ctx context.Context, optionFns ...RequestOption) (*Response, error) {
	return e.doRequest(ctx, http.MethodPost, optionFns...)
}

// Put will issue a post request to the endpoint.
func (e *Endpoint) Put(ctx context.Context, optionFns ...RequestOption) (*Response, error) {
	return e.doRequest(ctx, http.MethodPut, optionFns...)
}

// Delete  will issue a delete request to the endpoint.
func (e *Endpoint) Delete(ctx context.Context, optionFns ...RequestOption) (*Response, error) {
	return e.doRequest(ctx, http.MethodDelete, optionFns...)
}

// Patch will issue a patch request to the endpoint.
func (e *Endpoint) Patch(ctx context.Context, optionFns ...RequestOption) (*Response, error) {
	return e.doRequest(ctx, http.MethodPatch, optionFns...)
}

func (e *Endpoint) doRequest(ctx context.Context, method string, opts ...RequestOption) (*Response, error) {
	options := defaultRequestOptions()

	for _, option := range opts {
		option.applyRequest(&options)
	}

	if options.TargetID != "" {
		ctx = tracing.WithTargetID(ctx, options.TargetID)
	} else if e.targetID != "" {
		ctx = tracing.WithTargetID(ctx, e.targetID)
	}

	ctx = tracing.WithEndpointTemplate(ctx, e.formatURL.Path)

	targetURL, err := expandURLTemplate(e.formatURL, options.Params, options.Query)
	if err != nil {
		return nil, err
	}

	requestHeaders := make(http.Header, len(e.defaultHeaders)+len(options.Header))
	copyHeader(requestHeaders, e.defaultHeaders)
	copyHeader(requestHeaders, options.Header)

	body, err := getBody(options.RequestBody, requestHeaders)
	if err != nil {
		return nil, err
	}

	request, err := httpclient.NewRequest(ctx, method, targetURL.String(), body)
	if err != nil {
		return nil, err
	}

	request.Header = requestHeaders

	// If the user does not provide a User-Agent we set a default.
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "restclient-go/"+internal.Version)
	}

	ctx, span := newSpan(request)
	defer span.End()

	request = request.WithContext(ctx)
	response, err := e.requester.Do(request)
	recordResponseAttributes(span, response, err)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	r := Response{
		Body:       b,
		StatusCode: response.StatusCode,
		Header:     response.Header,
	}

	return &r, e.errorPolicy(&r)
}

func getBody(body any, headers http.Header) (any, error) {
	switch t := body.(type) {
	case io.Reader, nil, []byte:
		return t, nil

	default:
		ct, _, err := mime.ParseMediaType(headers.Get("Content-Type"))
		if err != nil {
			return nil, ErrUnsupportedBodyType
		}

		var content []byte

		switch ct {
		case "application/json":
			content, err = json.Marshal(body)
		default:
			return nil, ErrUnsupportedBodyType
		}

		if err != nil {
			return nil, err
		}

		return content, nil
	}
}

func copyHeader(dst, src http.Header) {
	for k := range src {
		dst.Set(k, src.Get(k))
	}
}
