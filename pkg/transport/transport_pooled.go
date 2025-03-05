package transport

import (
	"expvar"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/luizaranda/go-core/pkg/telemetry/dialtrace"
)

const (
	_expvarPrefix = "toolkit.http.client.conn_pools"
)

var (
	_expvar = expvar.NewMap(_expvarPrefix)
)

// NewPooled creates an *http.Transport with the given options and decorates its
// net.Dialer returning a PooledTransport which provides insight on the number
// of opened connections per network address.
func NewPooled(name string, opts ...Option) *PooledTransport {
	return NewPooledFromTransport(name, NewTransport(opts...))
}

// NewPooledFromTransport wraps an *http.Transport and decorates its net.Dialer
// returning a PooledTransport which provides insight on the number of
// opened connections per network address.
func NewPooledFromTransport(name string, transport *http.Transport) *PooledTransport {
	t := &PooledTransport{
		Transport: transport,
		Name:      name,
	}

	t.DialContext = dialtrace.NewTracedDialer(t.DialContext, dialtrace.DialerTrace{
		GotConn:   t.traceConn(1),
		CloseConn: t.traceConn(-1),
	})

	t.registerExpVar()

	return t
}

// PooledTransport is an implementation of an http.RoundTripper which provides
// insight on the number of connections it has opened per network address.
type PooledTransport struct {
	*http.Transport

	Name  string
	stats sync.Map
}

func (t *PooledTransport) traceConn(delta int64) func(network, address string) {
	return func(network, address string) {
		key := dialTraceKey(network, address)
		value, _ := t.stats.LoadOrStore(key, new(int64))
		atomic.AddInt64(value.(*int64), delta)
	}
}

func dialTraceKey(network, address string) string { return network + ":" + address }

// Stats returns transport statistics.
func (t *PooledTransport) Stats() map[string]int64 {
	stats := map[string]int64{}

	t.stats.Range(func(key, value interface{}) bool {
		stats[key.(string)] = atomic.LoadInt64(value.(*int64))
		return true
	})

	return stats
}

func (t *PooledTransport) registerExpVar() {
	f := func() interface{} { return t.Stats() }
	_expvar.Set(t.Name, expvar.Func(f))
}
