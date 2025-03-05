package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Supported MIME Content-Types.
const (
	_mimeApplicationJSON = "application/json"
)

var _validate = validator.New()

// DecodeJSON deserializes a request body into the given destination.
//
// This function may invoke data validation after deserialization.
func DecodeJSON(r *http.Request, destination interface{}) error {
	// We default to application/json if content type is not specified but return
	// http.StatusUnsupportedMediaType if it's specified but not supported.
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = _mimeApplicationJSON
	}

	switch {
	case strings.HasPrefix(ct, _mimeApplicationJSON):
		return decodeJSON(r.Context(), r.Body, destination)
	default:
		return NewErrorf(http.StatusUnsupportedMediaType, "unsupported media type: %s", ct)
	}
}

func decodeJSON(ctx context.Context, r io.Reader, destination interface{}) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(destination); err != nil {
		return handleDecodeErr(err)
	}

	if err := _validate.StructCtx(ctx, destination); err != nil {
		return handleValidateErr(err)
	}

	return nil
}

func handleDecodeErr(err error) error {
	switch e := err.(type) {
	case *json.InvalidUnmarshalError:
		return NewErrorf(400, "invalid_unmarshal_error: expected=%v", e.Type)
	case *json.UnmarshalTypeError:
		return NewErrorf(400,
			"unmarshal_type_error: expected=%v, got=%v, field=%v, offset=%v", e.Type, e.Value, e.Field, e.Offset)
	case *json.SyntaxError:
		return NewErrorf(400, "syntax_error: offset=%v, error=%v", e.Offset, e)
	default:
		return NewErrorf(400, err.Error())
	}
}

func handleValidateErr(err error) error {
	var invalidValidationError *validator.InvalidValidationError
	if errors.As(err, &invalidValidationError) {
		// We choose to ignore errors related to types
		// that can't be validated like time.Time and slices.
		return nil
	}

	message := err.Error()

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fields := make([]string, 0, len(validationErrs))
		for _, v := range validationErrs {
			fields = append(fields, v.Field())
		}
		message = fmt.Sprintf("invalid fields: %s", strings.Join(fields, ","))
	}

	return NewErrorf(http.StatusUnprocessableEntity, "validation_error: %s", message)
}
