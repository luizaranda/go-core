package transport

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

var (
	// DefaultDialTimeout is the max interval of time the dialer will wait when
	// executing the TCP handshake before returning a timeout error.
	//
	// This value is known and fixed within the internal network.
	DefaultDialTimeout = 300 * time.Millisecond

	// DefaultKeepAliveProbeInterval is the interval at which the dialer sets the
	// KeepAlive probe packet to be sent to assert the state of the connection.
	DefaultKeepAliveProbeInterval = 15 * time.Second
)

// An Option configures a http.Transport.
type Option interface {
	applyTransport(*http.Transport)
	applyDialer(*net.Dialer)
}

type transportOptFunc func(*http.Transport)

func (f transportOptFunc) applyTransport(t *http.Transport) { f(t) }
func (f transportOptFunc) applyDialer(*net.Dialer)          {}

type dialerOptFunc func(*net.Dialer)

func (f dialerOptFunc) applyTransport(*http.Transport) {}
func (f dialerOptFunc) applyDialer(d *net.Dialer)      { f(d) }

// OptionDialTimeout sets the timeout of the transports net.Dialer.
func OptionDialTimeout(timeout time.Duration) Option {
	return dialerOptFunc(func(d *net.Dialer) {
		d.Timeout = timeout
	})
}

// OptionResponseHeaderTimeout sets the ResponseHeaderTimeout of the transport.
func OptionResponseHeaderTimeout(timeout time.Duration) Option {
	return transportOptFunc(func(t *http.Transport) {
		t.ResponseHeaderTimeout = timeout
	})
}

// OptionExpectContinueTimeout sets the ExpectContinueTimeout of the transport.
func OptionExpectContinueTimeout(timeout time.Duration) Option {
	return transportOptFunc(func(t *http.Transport) {
		t.ExpectContinueTimeout = timeout
	})
}

// OptionIdleConnTimeout sets the IdleConnTimeout of the transport.
func OptionIdleConnTimeout(timeout time.Duration) Option {
	return transportOptFunc(func(t *http.Transport) {
		t.IdleConnTimeout = timeout
	})
}

// OptionTLSHandshakeTimeout sets the TLSHandshakeTimeout of the transport.
func OptionTLSHandshakeTimeout(timeout time.Duration) Option {
	return transportOptFunc(func(t *http.Transport) {
		t.TLSHandshakeTimeout = timeout
	})
}

// OptionTLSClientConfig allows setting the TLSClientConfig of the transport.
func OptionTLSClientConfig(config *tls.Config) Option {
	return transportOptFunc(func(t *http.Transport) {
		t.TLSClientConfig = config
	})
}

func NewTransport(opts ...Option) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   DefaultDialTimeout,
		KeepAlive: DefaultKeepAliveProbeInterval,
		DualStack: true,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   500,
		Proxy:                 http.ProxyFromEnvironment,
		ExpectContinueTimeout: 1 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	for _, opt := range opts {
		opt.applyDialer(dialer)
		opt.applyTransport(transport)
	}

	return transport
}
