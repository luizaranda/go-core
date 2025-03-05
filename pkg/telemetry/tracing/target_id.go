package tracing

import (
	"context"
)

type targetIDCtxKey struct{}

// WithTargetID sets the given targetID in the context for telemetry purposes.
func WithTargetID(ctx context.Context, targetID string) context.Context {
	return context.WithValue(ctx, targetIDCtxKey{}, targetID)
}

// TargetID returns the targetID associated with the given context or empty
// is none is found.
//
// TargetID can be set by using WithTargetID function.
func TargetID(ctx context.Context) string {
	value, _ := ctx.Value(targetIDCtxKey{}).(string)
	return value
}

type endpointTemplateKey struct{}

// WithEndpointTemplate sets the given endpoint template in the context for
// tracing purposes.
func WithEndpointTemplate(ctx context.Context, endpointTemplate string) context.Context {
	return context.WithValue(ctx, endpointTemplateKey{}, endpointTemplate)
}

// EndpointTemplate returns the endpoint template associated with the given
// context or empty is none is found.
//
// EndpointTemplate can be set by using WithEndpointTemplate function.
func EndpointTemplate(ctx context.Context) string {
	value, _ := ctx.Value(endpointTemplateKey{}).(string)
	return value
}
