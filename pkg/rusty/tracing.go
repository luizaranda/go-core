package rusty

import (
	"context"
	"fmt"
	"net/http"

	"github.com/luizaranda/go-core/pkg/internal"
	"github.com/luizaranda/go-core/pkg/telemetry/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	_instrumentationName = "github.com/luizaranda/go-core/pkg/rusty"
	_rustySpanName       = "RestClient"

	_endpointSpanAttribute = attribute.Key("toolkits.services.restclient.endpoint_rusty")
	_retriesSpanAttribute  = attribute.Key("toolkits.services.restclient.retries")
)

func newSpan(req *http.Request) (context.Context, trace.Span) {
	tracer := otel.Tracer(_instrumentationName, trace.WithInstrumentationVersion(internal.Version))

	ctx, span := tracer.Start(req.Context(), spanName(req.Method))
	span.SetAttributes(semconv.HTTPClientAttributesFromHTTPRequest(req)...)
	span.SetAttributes(_endpointSpanAttribute.String(tracing.EndpointTemplate(ctx)))

	return ctx, span
}

func recordResponseAttributes(span trace.Span, res *http.Response, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	if retries := res.Request.Header.Get("x-retry"); retries != "" {
		span.SetAttributes(_retriesSpanAttribute.String(retries))
	}

	span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(res.StatusCode)...)
	span.SetStatus(semconv.SpanStatusFromHTTPStatusCode(res.StatusCode))
}

func spanName(method string) string {
	return fmt.Sprintf("%s %s", _rustySpanName, method)
}
