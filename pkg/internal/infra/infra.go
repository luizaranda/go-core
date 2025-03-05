package infra

import (
	"context"
	"expvar"
	"net"
	"net/http"
	"net/http/pprof"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/luizaranda/go-core/pkg/log"
	"github.com/luizaranda/go-core/pkg/telemetry"
	"github.com/luizaranda/go-core/pkg/web"
	"go.opentelemetry.io/otel"
)

// Config structure that is used to configure a new Application.
type Config struct {
	ErrorHandler          web.ErrorHandler
	ErrorEncoder          web.ErrorEncoder
	NotFoundHandler       http.Handler
	HealthCheckRegisterer func(r *web.Router)

	DisableCompression bool
	Logger             log.Logger
	Tracer             telemetry.Client
	Network            string
	Address            string
	ServerTimeouts     web.Timeouts
	EnableProfiling    bool
}

const (
	// Default compression level for defined response content types.
	// The level should be one of the ones defined in the flate package.
	// Higher levels typically run slower but compress more.
	_defaultCompressionLevel = 5
)

type Application struct {
	*web.Router

	config Config

	Logger log.Logger
	Tracer telemetry.Client
}

// NewWebApplication instantiates an Application using the given configuration.
func NewWebApplication(config Config) (*Application, error) {
	// Set telemetry and logger package level defaults to the ones we've just
	// instantiated. This helps some users access log and telemetry without
	// having to manually propagate them as dependencies.
	log.DefaultLogger = config.Logger
	telemetry.DefaultTracer = config.Tracer

	router := defaultRouter(config)

	app := Application{
		config: config,
		Logger: config.Logger,
		Router: router,
		Tracer: config.Tracer,
	}

	return &app, nil
}

func defaultRouter(config Config) *web.Router {
	router := web.New()

	if config.NotFoundHandler != nil {
		router.NotFound(config.NotFoundHandler.ServeHTTP)
	}

	if config.ErrorHandler != nil {
		router.ErrorHandler(config.ErrorHandler)
	}

	if config.ErrorEncoder != nil {
		router.ErrorEncoder(config.ErrorEncoder)
	}

	// We register the health check handler before middlewares to avoid sending data about pings to
	// our telemetry providers.
	if config.HealthCheckRegisterer != nil {
		config.HealthCheckRegisterer(router)
	}

	// Enable OpenTelemetry middleware for sending telemetry data associated to the lifecycle of a http request.
	// GetTracerProvider returns the registered global trace provider that is set using https://github.com/luizaranda/go-core/pkg/otel.
	// Otherwise, a NoopTracerProvider is returned.
	router.Use(web.OpenTelemetry(web.OtelConfig{Provider: otel.GetTracerProvider(), MetricProvider: otel.GetMeterProvider()}))

	if config.EnableProfiling {
		g := router.Group("/debug", wrapM(middleware.NoCache))

		g.Get("/", func(w http.ResponseWriter, r *http.Request) error {
			http.Redirect(w, r, r.RequestURI+"/pprof/", http.StatusMovedPermanently)
			return nil
		})

		g.Any("/pprof", func(w http.ResponseWriter, r *http.Request) error {
			http.Redirect(w, r, r.RequestURI+"/", http.StatusMovedPermanently)
			return nil
		})

		g.Any("/pprof/*", wrapF(pprof.Index))
		g.Any("/pprof/cmdline", wrapF(pprof.Cmdline))
		g.Any("/pprof/profile", wrapF(pprof.Profile))
		g.Any("/pprof/symbol", wrapF(pprof.Symbol))
		g.Any("/pprof/trace", wrapF(pprof.Trace))
		g.Any("/vars", wrapF(expvar.Handler().ServeHTTP))

		g.Any("/pprof/goroutine", wrapF(pprof.Handler("goroutine").ServeHTTP))
		g.Any("/pprof/threadcreate", wrapF(pprof.Handler("threadcreate").ServeHTTP))
		g.Any("/pprof/mutex", wrapF(pprof.Handler("mutex").ServeHTTP))
		g.Any("/pprof/heap", wrapF(pprof.Handler("heap").ServeHTTP))
		g.Any("/pprof/block", wrapF(pprof.Handler("block").ServeHTTP))
		g.Any("/pprof/allocs", wrapF(pprof.Handler("allocs").ServeHTTP))
	}

	router.Use(
		web.Telemetry(config.Tracer),
		web.Logger(config.Logger),
		web.Panics(),
		web.HeaderForwarder())

	if !config.DisableCompression {
		router.Use(newCompressor())
	}

	return router
}

func wrapF(h http.HandlerFunc) web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		h(w, r)
		return nil
	}
}

func wrapM(m func(h http.Handler) http.Handler) web.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return m(next).ServeHTTP
	}
}

// newCompressor returns a middleware that compresses response body of a given content type to a data format based
// on Accept-Encoding request header. It uses the _defaultCompressionLevel.
//
// NOTE: if you don't use web.EncodeJSON to marshal the body into the writer,
// make sure to set the Content-Type header on your response otherwise this middleware will not compress the response body.
func newCompressor() web.Middleware {
	c := middleware.NewCompressor(_defaultCompressionLevel)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return c.Handler(next).ServeHTTP
	}
}

func RunListener(ctx context.Context, ln net.Listener, tracer telemetry.Client, logger log.Logger, timeouts web.Timeouts, r *web.Router) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go exportedVarPolling(ctx, tracer)

	logger.Info("running", log.String("address", ln.Addr().String()))

	if err := web.RunWithContext(ctx, ln, timeouts, r); err != nil && err != http.ErrServerClosed {
		return err
	}

	// From this point onwards we are on "clean-up" state.
	return tracer.Close()
}
