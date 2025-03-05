package transport

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luizaranda/go-core/pkg/telemetry"
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	// Metrics recorded when a new connection is established.
	_httpDNSTimingMetric          = "toolkit.http.client.dns.time"
	_httpTCPConnectTimingMetric   = "toolkit.http.client.tcp_connect.time"
	_httpTLSHandshakeTimingMetric = "toolkit.http.client.tls_handshake.time"

	// Metric recorded when a request connection is obtained.
	_httpConnectionGotTimingMetric = "toolkit.http.client.got_connection.time"

	// HTTP Request/Response timing metrics.
	_httpRequestMetric                    = "toolkit.http.client.request.time"
	_httpWroteRequestTimingMetric         = "toolkit.http.client.request_written.time"
	_httpGotFirstResponseByteTimingMetric = "toolkit.http.client.response_first_byte.time"
	_httpResponseFullyReadTimingMetric    = "toolkit.http.client.response_fully_read.time"
)

// TraceDecorator returns a RoundTripDecorator that provides HTTP tracing
// capabilities to the given http.RoundTripper.
//
// For more information check TracedRoundTripper struct.
func TraceDecorator() RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &TracedRoundTripper{Transport: base}
	}
}

// TracedRoundTripper is a http.RoundTripper that instruments external requests
// adding NewRelic distributed tracing headers, and recording a single metric on
// request/response behavior.
//
// Metric is recorded using `pkg/telemetry`, so in order to have working
// metrics the request context must contain a valid telemetry.Client. Metrics
// can be made more granular by making the request context have a target_id, use
// `pkg/telemetry/tracing` for that.
//
// NewRelic's integration works only if the request context contains a NewRelic
// transaction (web or non-web).
type TracedRoundTripper struct {
	Transport http.RoundTripper
}

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
func (t *TracedRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	// Start NewRelic external segment manually instead of using their round
	// tripper as we want to configure additional segment fields.
	// Note: this call mutates req. Refer to NewRelic docs for more information.
	segment := newrelic.StartExternalSegment(nil, request)
	segment.Procedure = buildSegmentProcedure(request)

	commonTags := tracedCommonTags(request)
	startTime := time.Now()

	// At last, we RoundTrip de request into the wrapped transport.
	response, err := t.Transport.RoundTrip(request)
	if err != nil {
		segment.AddAttribute("error", err.Error())
	}
	segment.Response = response
	segment.End()

	// When Transport.RoundTrip returns it means we have finished making the
	// request, either successfully or with error. The following method will
	// record a request metric with information about the response status, which
	// is either the response status code, a timeout or an unknown error.
	recordResponse(request.Context(), commonTags, startTime, _httpRequestMetric, response, err)

	return response, err
}

// ExtendedTraceDecorator returns a RoundTripDecorator that provides HTTP tracing
// capabilities to the given http.RoundTripper.
//
// For more information check ExtendedTracedRoundTripper struct.
func ExtendedTraceDecorator() RoundTripDecorator {
	return func(base http.RoundTripper) http.RoundTripper {
		return &ExtendedTracedRoundTripper{Transport: base}
	}
}

// ExtendedTracedRoundTripper is a http.RoundTripper that instruments external requests
// adding NewRelic distributed tracing headers, and recording various metrics on
// request/response behavior.
//
// Metrics are recorded using `pkg/telemetry`, so in order to have working
// metrics the request context must contain a valid telemetry.Client. Metrics
// can be made more granular by making the request context have a target_id, use
// `pkg/telemetry/tracing` for that.
//
// NewRelic's integration works only if the request context contains a NewRelic
// transaction (web or non-web).
type ExtendedTracedRoundTripper struct {
	Transport http.RoundTripper
}

// RoundTrip executes a single HTTP transaction, returning
// a Response for the provided Request.
func (t *ExtendedTracedRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	// Start NewRelic external segment manually instead of using their round
	// tripper as we want to configure additional segment fields.
	// Note: this call mutates req. Refer to NewRelic docs for more information.
	segment := newrelic.StartExternalSegment(nil, request)
	segment.Procedure = buildSegmentProcedure(request)

	commonTags := tracedCommonTags(request)
	startTime := time.Now()
	extendedTracedRequest := newTracedRequest(request, commonTags, startTime)

	// At last, we RoundTrip de request into the wrapped transport.
	response, err := t.Transport.RoundTrip(extendedTracedRequest)
	if err != nil {
		segment.AddAttribute("error", err.Error())
	} else {
		// Body is read outside the RoundTrip method. We might not have a timeout reading
		// the headers but might reach a timeout when reading the response body.
		// We decorate the response Body with a traced implementation.
		response.Body = &errorReadCloser{
			R: response.Body,
			OnErr: func(err error) {
				if err == io.EOF {
					err = nil
				}
				recordResponse(request.Context(), commonTags, startTime, _httpResponseFullyReadTimingMetric, response, err)
			},
		}
	}
	segment.Response = response
	segment.End()

	// When Transport.RoundTrip returns it means we have finished making the
	// request, either successfully or with error. The following method will
	// record a request metric with information about the response status, which
	// is either the response status code, a timeout or an unknown error.
	recordResponse(request.Context(), commonTags, startTime, _httpRequestMetric, response, err)

	return response, err
}

func tracedCommonTags(req *http.Request) []string {
	targetID := tracing.TargetID(req.Context())

	if targetID == "" {
		return []string{
			"technology:go",
			"method:" + strings.ToLower(req.Method),
		}
	}

	return []string{
		"technology:go",
		"target_id:" + targetID,
		"method:" + strings.ToLower(req.Method),
	}
}

func buildSegmentProcedure(request *http.Request) string {
	ctx := request.Context()

	endpointTemplate := tracing.EndpointTemplate(ctx)
	if endpointTemplate != "" {
		return request.Method + " " + endpointTemplate
	}

	targetID := tracing.TargetID(ctx)
	if targetID != "" {
		return request.Method + " " + targetID
	}

	return ""
}

func recordResponse(ctx context.Context, tags []string, startTime time.Time, metric string, response *http.Response, err error) {
	status, statusClass := "error", "error"
	if err == nil {
		status = strconv.Itoa(response.StatusCode)
		statusClass = strconv.Itoa(response.StatusCode/100) + "xx" // 2xx, 3xx, 4xx, 5xx, etc
	} else if os.IsTimeout(err) {
		status = "timeout"
	}

	recordTimeSince(ctx, metric, startTime, append(tags, "status:"+status, "status_class:"+statusClass))
}

func newTracedRequest(request *http.Request, tags []string, startTime time.Time) *http.Request {
	ctx := request.Context()

	var (
		dnsStart          time.Time
		tlsHandshakeStart time.Time
		tcpConnectStart   time.Time
	)

	// ClientTrace will call the given callbacks (when applicable) in the following order.
	// GetConn =>
	//    if no conn in pool {
	// 	      DNSStart => DNSDone
	// 	      ConnectStart => ConnectDone
	//        TLSHandshakeStart => TLSHandshakeDone
	//    }
	//    GotConn
	//    WroteRequest
	//    GotFirstResponseByte
	tracer := &httptrace.ClientTrace{
		// Following callbacks set the start time of the various request stages.
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		ConnectStart: func(network, addr string) {
			tcpConnectStart = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsHandshakeStart = time.Now()
		},
		// Callbacks for gathering stats on request connection state.
		DNSDone: func(info httptrace.DNSDoneInfo) {
			recordTimeSince(ctx, _httpDNSTimingMetric, dnsStart, append(tags, statusTag(info.Err)))
		},
		ConnectDone: func(network, addr string, err error) {
			recordTimeSince(ctx, _httpTCPConnectTimingMetric, tcpConnectStart, append(tags, statusTag(err)))
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			recordTimeSince(ctx, _httpTLSHandshakeTimingMetric, tlsHandshakeStart, append(tags, statusTag(err)))
		},
		// GotConn is called just before starting sending the request body. GotConnInfo will tell
		// us whether the connection was reused (in which case no DNS, TLS or Connect was performed)
		// or if the connection was created from scratch, in which case Connect was done, but
		// potentially DNS and/or TLS where not.
		GotConn: func(info httptrace.GotConnInfo) {
			tags := append(tags,
				"reused:"+strconv.FormatBool(info.Reused),
				"was_idle:"+strconv.FormatBool(info.WasIdle))
			recordTimeSince(ctx, _httpConnectionGotTimingMetric, startTime, tags)
		},
		// The following callbacks are executed only if the connection phase returned successfully.
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			recordTimeSince(ctx, _httpWroteRequestTimingMetric, startTime, append(tags, statusTag(info.Err)))
		},
		GotFirstResponseByte: func() {
			recordTimeSince(ctx, _httpGotFirstResponseByteTimingMetric, startTime, tags)
		},
	}

	return request.WithContext(httptrace.WithClientTrace(ctx, tracer))
}

func statusTag(err error) string {
	if err == nil {
		return "status:ok"
	}

	if os.IsTimeout(err) {
		return "status:timeout"
	}

	return "status:error"
}

func recordTimeSince(ctx context.Context, metric string, start time.Time, tags []string) {
	if start.IsZero() {
		return
	}

	telemetry.Timing(ctx, metric, time.Since(start), tags)
}

// errorReadCloser is a wrapper around ReadCloser R that calls OnErr handler
// with any error returned by Read (even EOF).
type errorReadCloser struct {
	// Underlying ReadCloser.
	R io.ReadCloser

	// OnErr is called with the error (even EOF) returned by underlying Read.
	OnErr func(error)
}

// Read reads the next len(p) bytes from R or until R is drained. The
// return value n is the number of bytes read. If R has no data to
// return, err is io.EOF and OnEOF is called with a full copy of what
// has been read so far.
func (r *errorReadCloser) Read(p []byte) (n int, err error) {
	n, err = r.R.Read(p)
	if err != nil {
		r.OnErr(err)
	}
	return n, err
}

func (r *errorReadCloser) Close() error {
	return r.R.Close()
}
