package app

import (
	"net/http"

	"github.com/luizaranda/go-core/pkg/log"
	"github.com/luizaranda/go-core/pkg/web"
)

type Config struct {
	ErrorHandler    web.ErrorHandler
	ErrorEncoder    web.ErrorEncoder
	NotFoundHandler http.Handler

	DisableCompression bool
	LogLevel           log.Level
	LogOptions         []log.Option
	ServerTimeouts     web.Timeouts
	EnableProfiling    bool
}

// AppOptFunc allows defining custom functions for configuring an Application.
type AppOptFunc func(*Config)

// WithErrorHandler sets a custom error handling function that will process any error returned from a handler.
func WithErrorHandler(errHandler web.ErrorHandler) AppOptFunc {
	return func(config *Config) {
		config.ErrorHandler = errHandler
	}
}

// WithErrorEncoder sets a custom error encoder function that should be used for encoding
// the given error into the http.ResponseWriter.
// This means writing the header first and then the response body.
func WithErrorEncoder(errEncoder web.ErrorEncoder) AppOptFunc {
	return func(config *Config) {
		config.ErrorEncoder = errEncoder
	}
}

// WithNotFoundHandler sets a handler for routing paths that could not be found.
// Default is infra.NotFoundHandler.
func WithNotFoundHandler(h http.Handler) AppOptFunc {
	return func(config *Config) {
		config.NotFoundHandler = h
	}
}

// WithLogLevel sets the level at which the application logger will log.
//
// Default behavior is to log at Info level in production, and log level in
// local and test environments.
func WithLogLevel(level log.Level) AppOptFunc {
	return func(config *Config) {
		config.LogLevel = level
	}
}

// WithLogOptions sets the options to the application logger.
func WithLogOptions(opts ...log.Option) AppOptFunc {
	return func(config *Config) {
		config.LogOptions = opts
	}
}

// WithTimeouts sets the different timeouts that the web server uses.
//
// Default behavior is to not have timeouts for incoming requests.
func WithTimeouts(timeouts web.Timeouts) AppOptFunc {
	return func(config *Config) {
		config.ServerTimeouts = timeouts
	}
}

// WithEnableProfiling enables pprof handlers that exposes runtime diagnostic data.
func WithEnableProfiling() AppOptFunc {
	return func(config *Config) {
		config.EnableProfiling = true
	}
}

// WithDisableCompression disables the default compressor.
func WithDisableCompression() AppOptFunc {
	return func(config *Config) {
		config.DisableCompression = true
	}
}
