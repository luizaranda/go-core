package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// DefaultTimeouts exports sane timeouts for Run.
var DefaultTimeouts = Timeouts{
	ShutdownTimeout: 10 * time.Second,
}

// Timeouts struct define different timeouts that Run takes into consideration
// when running the web server.
type Timeouts struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body. A zero or negative value means
	// there will be no timeout.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	ReadTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If ReadHeaderTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	ReadHeaderTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	// A zero or negative value means there will be no timeout.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	IdleTimeout time.Duration

	// ShutdownTimeout is the maximum duration for the server
	// to gracefully shutdown.
	ShutdownTimeout time.Duration
}

// Run runs the handler h on the given net.Listener using a http.Server configured with the given
// timeouts.
// It blocks until SIGTERM o SIGINT is received by the running process.
func Run(ln net.Listener, timeouts Timeouts, h http.Handler) error {
	server := http.Server{
		ReadTimeout:       timeouts.ReadTimeout,
		ReadHeaderTimeout: timeouts.ReadHeaderTimeout,
		WriteTimeout:      timeouts.WriteTimeout,
		IdleTimeout:       timeouts.IdleTimeout,
		Handler:           h,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return run(ctx, &server, timeouts.ShutdownTimeout, ln)
}

// RunWithContext runs the handler h on the given net.Listener using a http.Server configured with the given
// timeouts.
// It blocks until the given context's Done channel is closed.
func RunWithContext(ctx context.Context, ln net.Listener, timeouts Timeouts, h http.Handler) error {
	server := http.Server{
		ReadTimeout:       timeouts.ReadTimeout,
		ReadHeaderTimeout: timeouts.ReadHeaderTimeout,
		WriteTimeout:      timeouts.WriteTimeout,
		IdleTimeout:       timeouts.IdleTimeout,
		Handler:           h,
	}

	return run(ctx, &server, timeouts.ShutdownTimeout, ln)
}

func run(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, ln net.Listener) error {
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(ln)
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("error in serve: %w", err)
	case <-ctx.Done():
		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			server.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
