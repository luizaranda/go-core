package infra

import (
	"context"
	"expvar"
	"net"
	"net/http"
	"net/http/pprof"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
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

		// Usando o método Handle diretamente para o redirecionamento
		router.Engine().GET("/debug", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, c.Request.RequestURI+"/pprof/")
		})

		router.Engine().Any("/debug/pprof", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, c.Request.RequestURI+"/")
		})

		// Usando o método Handle diretamente para evitar problemas com o método Any
		router.Engine().Any("/debug/pprof/*path", func(c *gin.Context) {
			path := c.Param("path")
			if path == "" {
				pprof.Index(c.Writer, c.Request)
				return
			}
			switch path {
			case "cmdline":
				pprof.Cmdline(c.Writer, c.Request)
			case "profile":
				pprof.Profile(c.Writer, c.Request)
			case "symbol":
				pprof.Symbol(c.Writer, c.Request)
			case "trace":
				pprof.Trace(c.Writer, c.Request)
			case "goroutine":
				pprof.Handler("goroutine").ServeHTTP(c.Writer, c.Request)
			case "threadcreate":
				pprof.Handler("threadcreate").ServeHTTP(c.Writer, c.Request)
			case "mutex":
				pprof.Handler("mutex").ServeHTTP(c.Writer, c.Request)
			case "heap":
				pprof.Handler("heap").ServeHTTP(c.Writer, c.Request)
			case "block":
				pprof.Handler("block").ServeHTTP(c.Writer, c.Request)
			case "allocs":
				pprof.Handler("allocs").ServeHTTP(c.Writer, c.Request)
			default:
				pprof.Index(c.Writer, c.Request)
			}
		})
		// Usando o método Handle diretamente para o expvar handler
		router.Engine().Any("/debug/vars", func(c *gin.Context) {
			expvar.Handler().ServeHTTP(c.Writer, c.Request)
		})

		// Nota: Os handlers específicos de pprof são tratados pela rota curinga /debug/pprof/*path
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

// wrapF adapts an http.HandlerFunc to the web.Handler format
func wrapF(h http.HandlerFunc) web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		h(w, r)
		return nil
	}
}

// wrapHTTPHandler adapta um http.Handler para o formato web.Handler
func wrapHTTPHandler(h http.Handler) web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		h.ServeHTTP(w, r)
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
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Gin has built-in compression in the engine
			// We're keeping this middleware for compatibility, but it's a no-op now
			next(w, r)
		}
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
