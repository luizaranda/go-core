package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    e.Code,
		Message: e.Message,
	})
}

// StatusCode returns the HTTP status code for the error.
func (e *Error) StatusCode() int {
	return e.Status
}

// Error returns a string message of the error, implementing the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError creates a new error with the given status code and message.
func NewError(statusCode int, message string) error {
	return NewErrorf(statusCode, message)
}

// NewErrorf creates a new error with a formatted message.
func NewErrorf(status int, format string, args ...interface{}) error {
	return &Error{
		Code:    strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_"),
		Message: fmt.Sprintf(format, args...),
		Status:  status,
	}
}

// BadRequestError returns a 400 Bad Request error.
func BadRequestError(message string) error {
	return NewError(http.StatusBadRequest, message)
}

// BadRequestErrorf returns a formatted 400 Bad Request error.
func BadRequestErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusBadRequest, format, args...)
}

// UnauthorizedError returns a 401 Unauthorized error.
func UnauthorizedError(message string) error {
	return NewError(http.StatusUnauthorized, message)
}

// UnauthorizedErrorf returns a formatted 401 Unauthorized error.
func UnauthorizedErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusUnauthorized, format, args...)
}

// ForbiddenError returns a 403 Forbidden error.
func ForbiddenError(message string) error {
	return NewError(http.StatusForbidden, message)
}

// ForbiddenErrorf returns a formatted 403 Forbidden error.
func ForbiddenErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusForbidden, format, args...)
}

// NotFoundError returns a 404 Not Found error.
func NotFoundError(message string) error {
	return NewError(http.StatusNotFound, message)
}

// NotFoundErrorf returns a formatted 404 Not Found error.
func NotFoundErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusNotFound, format, args...)
}

// InternalServerError returns a 500 Internal Server Error.
func InternalServerError(message string) error {
	return NewError(http.StatusInternalServerError, message)
}

// InternalServerErrorf returns a formatted 500 Internal Server Error.
func InternalServerErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusInternalServerError, format, args...)
}

// ServiceUnavailableError returns a 503 Service Unavailable error.
func ServiceUnavailableError(message string) error {
	return NewError(http.StatusServiceUnavailable, message)
}

// ServiceUnavailableErrorf returns a formatted 503 Service Unavailable error.
func ServiceUnavailableErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusServiceUnavailable, format, args...)
}
