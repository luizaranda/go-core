package rusty

import (
	"fmt"
	"net/http"
	"strings"
)

// Error represents an application error from the API server.
// It is returned by the DefaultErrorPolicy policy.
type Error struct {
	// Response is the server response that caused this error. It is always non-nil.
	*Response
}

// Error implements the error interface.
func (e *Error) Error() string {
	code := strings.ReplaceAll(strings.ToLower(http.StatusText(e.StatusCode)), " ", "_")
	return fmt.Sprintf("%d %s: %s", e.StatusCode, code, string(e.Body))
}
