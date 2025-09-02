package web

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
)

type uriParamsContextKey struct{}

// URIParams contains the key-value combination of parameters from the URI.
// Deprecated: use web.Param and web.ParamInt instead since this type will be removed in a future release.
type URIParams map[string]string

// Params returns a map from the given request's context containing every URI parameter defined in the route.
// The key represents the name of the route variable for the current request, if any.
// Deprecated: use web.Param instead since this method allocates a new map on every call.
// Also, note that the error returned by web.Param is different from the one returned by web.Params.
// Users should update their error handling accordingly.
// This function will be removed in a future release.
func Params(r *http.Request) URIParams {
	// This is for backward compatibility since the router no longer stores the params in the request context.
	// It supports the usage of web.WithParams, which is also deprecated, in tests.
	if v, ok := r.Context().Value(uriParamsContextKey{}).(URIParams); ok {
		return v
	}

	chiCtx := chi.RouteContext(r.Context())
	if chiCtx == nil {
		return nil
	}

	routeParams := chiCtx.URLParams
	params := make(URIParams, len(routeParams.Keys))
	for i := range routeParams.Keys {
		params[routeParams.Keys[i]] = routeParams.Values[i]
	}

	return params
}

// WithParams returns a new Context that carries the provided params.
// Deprecated: use web.WithURLParams instead. This function will be in a future release.
func WithParams(ctx context.Context, params URIParams) context.Context {
	return context.WithValue(ctx, uriParamsContextKey{}, params)
}

// String gets parameter as a string.
// If the parameter is not found, it returns InternalServerError(500).
func (p URIParams) String(param string) (string, error) {
	v, ok := p[param]
	if !ok {
		return "", NewErrorf(http.StatusInternalServerError, "uri param is not found: %s", param)
	}

	return v, nil
}

// Int gets parameter as an int.
// If the parameter is not found, it returns InternalServerError(500).
// If the parameter type value is a not an int, it returns BadRequestError(400).
func (p URIParams) Int(param string) (int, error) {
	v, ok := p[param]
	if !ok {
		return 0, NewErrorf(http.StatusInternalServerError, "uri param is not found: %s", param)
	}

	paramParsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, NewErrorf(http.StatusBadRequest, "uri param %s is not an int value: %s", param, v)
	}

	return paramParsed, nil
}

// Uint gets parameter as an uint.
// If the parameter is not found, it returns InternalServerError(500).
// If the parameter type value is a not an uint, it returns BadRequestError(400).
func (p URIParams) Uint(param string) (uint, error) {
	v, ok := p[param]
	if !ok {
		return 0, NewErrorf(http.StatusInternalServerError, "uri param is not found: %s", param)
	}

	paramParsed, err := strconv.ParseUint(v, 10, 0)
	if err != nil {
		return 0, NewErrorf(http.StatusBadRequest, "uri param %s is not an uint value: %s", param, v)
	}

	return uint(paramParsed), nil
}

// Bool gets parameter as a bool.
// If the parameter is not found, it returns InternalServerError(500).
// If the parameter type value is a not a bool, it returns BadRequestError(400).
func (p URIParams) Bool(param string) (bool, error) {
	v, ok := p[param]
	if !ok {
		return false, NewErrorf(http.StatusInternalServerError, "uri param is not found: %s", param)
	}

	parsedValue, err := strconv.ParseBool(v)
	if err != nil {
		return false, NewErrorf(http.StatusBadRequest, "uri param %s is not an bool value: %s", param, v)
	}

	return parsedValue, nil
}

// Param returns the value of the URL parameter with the given key.
// If the parameter is not found, it returns an empty string.
func Param(r *http.Request, key string) string {
	return chi.RouteContext(r.Context()).URLParam(key)
}

// ParamInt returns the value of the URL parameter with the given key as an int.
// If the parameter is not found, it returns 0.
// If the parameter type value is a not an int, it returns an error.
func ParamInt(r *http.Request, key string) (int, error) {
	value := Param(r, key)
	if value == "" {
		return 0, nil
	}

	intValue, err := strconv.Atoi(value)
	return intValue, err
}

// WithURLParams adds the given URL parameters to the request context.
// testing.T is required but not used to enforce the use of this function in tests only.
func WithURLParams(t *testing.T, req *http.Request, params map[string]string) *http.Request {
	if t == nil {
		panic("use WithURLParams only in tests")
	}

	var routeParams chi.RouteParams
	for key, val := range params {
		routeParams.Add(key, val)
	}

	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams = routeParams

	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	return req
}

// QueryParam returns the value of the query parameter with the given key.
// It returns an empty string if the parameter is not found.
func QueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// QueryParamInt returns the value of the query parameter with the given key as an int.
// It returns 0 if the parameter is not found or is not a valid integer.
// It returns an error if the parameter value is not an integer.
func QueryParamInt(r *http.Request, key string) (int, error) {
	value := QueryParam(r, key)
	if value == "" {
		return 0, nil
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return intValue, nil
}
