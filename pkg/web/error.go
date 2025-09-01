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

// StatusCode retorn HTTP status code for error
func (e *Error) StatusCode() int {
	return e.Status
}

// Error returns a string message of the error. It is a concatenation of Code and Message fields.
// This means the Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError creates a new error with the given status code and message.
func NewError(statusCode int, message string) error {
	return NewErrorf(statusCode, message)
}

// NewErrorf creates a new error with the given status code and the message
// formatted according to args and format.
func NewErrorf(status int, format string, args ...interface{}) error {
	return &Error{
		Code:    strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_"),
		Message: fmt.Sprintf(format, args...),
		Status:  status,
	}
}

// BadRequestError return 400 error
func BadRequestError(message string) error {
	return NewError(http.StatusBadRequest, message)
}

// BadRequestErrorf return 400 formated error
func BadRequestErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusBadRequest, format, args...)
}

// UnauthorizedError return 401 error
func UnauthorizedError(message string) error {
	return NewError(http.StatusUnauthorized, message)
}

// UnauthorizedErrorf return 401 formated error
func UnauthorizedErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusUnauthorized, format, args...)
}

// ForbiddenError return 403 error
func ForbiddenError(message string) error {
	return NewError(http.StatusForbidden, message)
}

// ForbiddenErrorf return 403 formated error
func ForbiddenErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusForbidden, format, args...)
}

// NotFoundError return 404 error
func NotFoundError(message string) error {
	return NewError(http.StatusNotFound, message)
}

// NotFoundErrorf return 404 formated error
func NotFoundErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusNotFound, format, args...)
}

// InternalServerError return 500 error
func InternalServerError(message string) error {
	return NewError(http.StatusInternalServerError, message)
}

// InternalServerErrorf return 500 formated error
func InternalServerErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusInternalServerError, format, args...)
}

// ServiceUnavailableError return 503 error
func ServiceUnavailableError(message string) error {
	return NewError(http.StatusServiceUnavailable, message)
}

// ServiceUnavailableErrorf return 503 formated error
func ServiceUnavailableErrorf(format string, args ...interface{}) error {
	return NewErrorf(http.StatusServiceUnavailable, format, args...)
}

// ValidationError represent validation error
type ValidationError struct {
	Error
	Details map[string]string `json:"details,omitempty"`
}

// NewValidationError creates new validation error
func NewValidationError(message string, details map[string]string) error {
	return &ValidationError{
		Error: Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "validation_error",
			Message: message,
		},
		Details: details,
	}
}
