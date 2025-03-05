package telemetry

import (
	"context"
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// Span contains a provider independent representation of a Segment.
// All spans have a beginning and end only when Finish is called.
// Ignore prevents this transaction's data from being recorded.
//
// Current implementation does not allow Spans to happen concurrently, this
// means that two currently running spans cannot have the same parent span.
// This results in undefined behavior.
type Span interface {
	// Finish ends the Span.
	Finish()

	// Ignore prevents this transaction's data from being recorded, depending on
	// implementation it may result in parent/child spans being ignored as well.
	Ignore()

	// SetLabel adds a key value pair to the transaction event, errors, and
	// traces.
	//
	// The key must contain fewer than 255 bytes. The value must be a
	// number, string, or boolean.
	SetLabel(key string, value interface{})

	// NoticeError traces an error to the Span. Depending on implementation
	// calling NoticeError multiple times may result in only the first, last or
	// all errors being recorded.
	NoticeError(err error)
}

// StartSpan begins a Span.
//   - This method never returns nil.
//   - If the given context contains a Span, then a child span is created and
//     assigned to the returned context.
//   - Caller must call Finish on the returned Span for recording to occur.
//   - Calling ignore may ignore a parent Span as well. This depends on each
//     provider.
//   - Calling SetLabel may modify existing labels on parent Spans. This depends
//     on each provider.
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	tx := newrelic.FromContext(ctx)
	if tx == nil {
		return DefaultTracer.StartSpan(ctx, name)
	}

	return ctx, &nrSegmentSpan{
		Transaction: tx,
		Segment:     tx.StartSegment(name),
	}
}

// StartAsyncSpan begins an asynchronous Span.
//   - This method never returns nil.
//   - If the given context contains a Span, then a child async span is created
//     and assigned to the returned context.
//   - Caller must call Finish on the returned Span for recording to occur.
//   - Calling ignore may ignore a parent Span as well. This depends on each
//     provider.
//   - Calling SetLabel may modify existing labels on parent Spans. This depends
//     on each provider.
//   - Each goroutine must have its own Span reference returned by StartAsyncSpan.
//     You must call StartAsyncSpan to get a new Span every time you wish to pass
//     the Span to another goroutine. It does not matter if you call this before
//     or after the other goroutine has started.
func StartAsyncSpan(ctx context.Context, name string) (context.Context, Span) {
	tx := newrelic.FromContext(ctx)
	if tx == nil {
		return DefaultTracer.StartSpan(ctx, name)
	}

	tx2 := tx.NewGoroutine()
	return newrelic.NewContext(ctx, tx2), &nrSegmentSpan{
		Transaction: tx2,
		Segment:     tx2.StartSegment(name),
	}
}

// nrTransactionSpan is a span that wraps a newrelic.Transaction, translating
// Span methods into the corresponding Transaction ones.
type nrTransactionSpan struct{ *newrelic.Transaction }

func (s *nrTransactionSpan) Ignore() { s.Transaction.Ignore() }
func (s *nrTransactionSpan) Finish() { s.Transaction.End() }
func (s *nrTransactionSpan) SetLabel(key string, value interface{}) {
	s.Transaction.AddAttribute(key, value)
}

// nrWebTransactionSpan is a span that wraps a newrelic.Transaction and the
// returned http.ResponseWriter resulting in calling SetWebResponse.
type nrWebTransactionSpan struct {
	http.ResponseWriter
	*nrTransactionSpan
}

var _ Span = (*nrWebTransactionSpan)(nil)

// nrSegmentSpan is a span that wraps a newrelic.Segment with its parent
// transaction. It translates Span methods into the corresponding Segment/
// Transaction ones.
type nrSegmentSpan struct {
	*newrelic.Transaction
	*newrelic.Segment
}

func (s *nrSegmentSpan) Finish() { s.Segment.End() }
func (s *nrSegmentSpan) Ignore() { s.Transaction.Ignore() }
func (s *nrSegmentSpan) SetLabel(key string, value interface{}) {
	s.Transaction.AddAttribute(key, value)
}

var _ Span = (*nrSegmentSpan)(nil)
