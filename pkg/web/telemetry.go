package web

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func notifyErr(ctx context.Context, err error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.NoticeError(err)
	}
}
