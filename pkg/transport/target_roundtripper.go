package transport

import (
	"net/http"

	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
)

// TargetDecorator returns a RoundTripDecorator that provides tagging HTTP
// requests with a target_id for tracing purposes.
//
// For more information check TargetRoundTripper struct.
func TargetDecorator(targetID string) RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &TargetRoundTripper{
			Transport: base,
			TargetID:  targetID,
		}
	}
}

// TargetRoundTripper simplifies the process of tagging all handled requests
// by the given RoundTripper with a TargetID for tracing purposes.
//
// If the request already contain a TargetID then it's used.
type TargetRoundTripper struct {
	Transport http.RoundTripper
	TargetID  string
}

func (t *TargetRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	target := tracing.TargetID(req.Context())
	if target == "" {
		ctx := tracing.WithTargetID(req.Context(), t.TargetID)
		req = req.WithContext(ctx)
	}
	return t.Transport.RoundTrip(req)
}
