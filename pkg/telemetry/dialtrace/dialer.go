package dialtrace

import (
	"context"
	"net"
)

// DialContextFunc is the interface that wraps the net.Dialer DialContext method.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// DialContext is the interface that wraps the net.Dialer DialContext method.
func (d DialContextFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d(ctx, network, address)
}

// NewTracedDialer returns a new DialContext based on the provided parent
// dialer. Dial operations made with the returned dialer will use
// the provided trace hooks.
func NewTracedDialer(dial DialContextFunc, trace DialerTrace) DialContextFunc {
	return (&tracedDialer{
		dial:  dial,
		trace: trace,
	}).DialContext
}

// DialerTrace is a set of hooks to run at various stages of a dial operation.
// Any particular hook may be nil. Functions may be called concurrently
// from different goroutines.
type DialerTrace struct {
	// GotConn is called after a successful connection is obtained.
	GotConn func(network, address string)

	// ConnError is called a failed attempt to establish a connection happens.
	ConnError func(network, address string, err error)

	// CloseConn is called after a connection is closed.
	CloseConn func(network, address string)
}

// A tracedDialer contains options for wrapping a dialer DialContext func
// with a DialTrace with callbacks to be called on connection events.
type tracedDialer struct {
	dial  DialContextFunc
	trace DialerTrace
}

// DialContext connects to the address on the named network using
// the provided context.
//
// The provided Context must be non-nil. If the context expires before
// the connection is complete, an error is returned. Once successfully
// connected, any expiration of the context will not affect the
// connection.
//
// When using TCP, and the host in the address parameter resolves to multiple
// network addresses, any dial timeout (from d.Timeout or ctx) is spread
// over each consecutive dial, such that each is given an appropriate
// fraction of the time to connect.
// For example, if a host has 4 IP addresses and the timeout is 1 minute,
// the connect to each single address will be given 15 seconds to complete
// before trying the next one.
//
// See func Dial for a description of the network and address
// parameters.
func (d *tracedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dial(ctx, network, address)
	if err != nil {
		if d.trace.ConnError != nil {
			d.trace.ConnError(network, address, err)
		}
		return nil, err
	}

	if d.trace.GotConn != nil {
		d.trace.GotConn(network, address)
	}

	return &tracedConn{
		Conn: conn,
		closeFunc: func() {
			if d.trace.CloseConn != nil {
				d.trace.CloseConn(network, address)
			}
		},
	}, nil
}

type tracedConn struct {
	net.Conn

	closeFunc func()
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *tracedConn) Close() error {
	// Call the close func after closing the connection.
	defer c.closeFunc()

	return c.Conn.Close()
}
