package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/luizaranda/go-core/pkg/internal/infra"
	"github.com/luizaranda/go-core/pkg/log"
	"github.com/luizaranda/go-core/pkg/otel"
	"github.com/luizaranda/go-core/pkg/telemetry"
	"github.com/luizaranda/go-core/pkg/web"
)

const (
	_defaultWebApplicationPort = "8080"
	_defaultScopeEnvironment   = "local"

	_otelAgentEnabledEnv  = "OTEL_AGENT_ENABLED"
	_otelAgentDisabledEnv = "OTEL_AGENT_DISABLED"
)

// Application is a container struct that contains all required base components for building web applications.
type Application struct {
	*web.Router
	Scope  Scope
	Tracer telemetry.Client
	Logger log.Logger

	mutex sync.Mutex // guards port
	port  int

	running chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc

	// Fields that contains information for running the application.
	network        string
	address        string
	serverTimeouts web.Timeouts

	otelShutdownFunc otel.ShutdownFunc
}

// Scope struct is the parsed representation of the value of the SCOPE in which the application is running.
type Scope struct {
	Environment string
	Role        string
	Metadata    string
}

// NewWebApplication instantiates an Application using the given configuration.
// Sane defaults are provided.
func NewWebApplication(opts ...AppOptFunc) (*Application, error) {
	var config Config
	for _, opt := range opts {
		opt(&config)
	}

	if config.LogLevel == 0 {
		config.LogLevel = log.InfoLevel
	}

	if config.ServerTimeouts == (web.Timeouts{}) {
		config.ServerTimeouts = web.Timeouts{
			IdleTimeout:     75 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		}
	}

	// We must start OTel before any other dependency since
	// there are components that require the global provider to be set.
	otelShutdownFunc, err := startOTel()
	if err != nil {
		return nil, err
	}

	scopeFromEnv := getScopeFromEnv()

	scope, err := infra.ParseScope(scopeFromEnv)
	if err != nil {
		return nil, err
	}

	tracer, err := newTracer(scope)
	if err != nil {
		return nil, err
	}

	logger, level := newLogger(config)

	port := os.Getenv("PORT")
	if port == "" {
		port = _defaultWebApplicationPort
	}

	cfg := infra.Config{
		ErrorHandler:       config.ErrorHandler,
		ErrorEncoder:       config.ErrorEncoder,
		NotFoundHandler:    config.NotFoundHandler,
		Logger:             logger,
		Tracer:             tracer,
		EnableProfiling:    config.EnableProfiling,
		DisableCompression: config.DisableCompression,
		ServerTimeouts:     config.ServerTimeouts,
	}

	cfg.HealthCheckRegisterer = func(r *web.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) error {
			return web.EncodeJSON(w, "pong", 200)
		})
	}

	app, err := infra.NewWebApplication(cfg)
	if err != nil {
		return nil, err
	}

	// Register logger handler for changing log level dynamically
	app.Router.Any("/debug/log/level", wrapF(level.ServeHTTP))

	// Context that will be canceled when calling Shutdown.
	ctx, cancel := context.WithCancel(context.Background())

	return &Application{
		Scope:  Scope(scope),
		Router: app.Router,
		Tracer: app.Tracer,
		Logger: app.Logger,

		network:          "tcp",
		address:          ":" + port,
		running:          make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
		serverTimeouts:   cfg.ServerTimeouts,
		otelShutdownFunc: otelShutdownFunc,
	}, nil
}

// Run starts your Application using a predefined network and address.
// It blocks until SIGTERM o SIGINT is received by the running process or Shutdown is called, whichever happens first.
func (a *Application) Run() error {
	defer func() { _ = a.otelShutdownFunc() }()

	ln, err := net.Listen(a.network, a.address)
	if err != nil {
		return err
	}

	a.mutex.Lock()
	// Once assigned, the application is ready to enqueue SYN messages.
	a.port = ln.Addr().(*net.TCPAddr).Port
	a.mutex.Unlock()

	close(a.running)
	return infra.RunListener(a.ctx, ln, a.Tracer, a.Logger, a.serverTimeouts, a.Router)
}

// Running returns a channel to signal a caller that the Application is ready to receive a SYN packet.
// Since Run is a blocking operation, this method comes handy specially when executing tests.
// Example:
//
//	func Test_App(t *testing.T) {
//		// Given
//		t.Setenv("PORT", "0")
//		app, err := app.NewWebApplication()
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		go app.Run()
//		<-app.Running()
//
//		// When
//		r, err := http.Get(fmt.Sprintf("http://localhost:%d/ping", app.Port()))
//
//		// Then
//		...
//	}
func (a *Application) Running() chan struct{} {
	return a.running
}

// Port returns the port number where this application is running.
func (a *Application) Port() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.port
}

// Shutdown shutdowns the application.
// Run method will return once all ongoing requests have been handled by the server
// or the ShutdownTimeout is reached.
func (a *Application) Shutdown() {
	a.cancel()
}

func getScopeFromEnv() string {
	scope := os.Getenv("SCOPE")
	if scope == "" {
		scope = _defaultScopeEnvironment
	}

	return scope
}

func newLogger(cfg Config) (log.Logger, *log.AtomicLevel) {
	l := log.NewAtomicLevelAt(cfg.LogLevel)
	return log.NewProductionLogger(&l, cfg.LogOptions...), &l
}

func newTracer(scope infra.Scope) (telemetry.Client, error) {
	tracer := telemetry.NewNoOpClient()
	if !strings.EqualFold(scope.Environment, _defaultScopeEnvironment) {
		t, err := telemetry.NewClient(newTelemetryConfig())
		if err != nil {
			return nil, err
		}
		tracer = t
	}

	return tracer, nil
}

func newTelemetryConfig() telemetry.Config {
	return telemetry.Config{
		ApplicationName:      os.Getenv("NEW_RELIC_APP_NAME"),
		NewRelicLicense:      os.Getenv("NEW_RELIC_LICENSE_KEY"),
		DatadogAddress:       "datadog:8125",
		NewRelicHighSecurity: false,
	}
}

func wrapF(h http.HandlerFunc) web.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		h(w, r)
		return nil
	}
}

func startOTel() (otel.ShutdownFunc, error) {
	if isOpenTelemetryEnabled() {
		shutdown, err := otel.Start(context.Background())
		if err != nil {
			return nil, err
		}

		return shutdown, nil
	}

	return func() error { return nil }, nil
}

func isOpenTelemetryEnabled() bool {
	return strings.EqualFold(os.Getenv(_otelAgentEnabledEnv), "true") &&
		!strings.EqualFold(os.Getenv(_otelAgentDisabledEnv), "true")
}
