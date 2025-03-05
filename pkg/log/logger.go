package log

import (
	"io"
	"os"
	"time"

	"github.com/luizaranda/go-core/pkg/log/encoders"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DefaultLogger is the default logger and is used when given a context
// with no associated log instance.
//
// DefaultLogger by default discards all logs. You can change its implementation
// by settings this variable to an instantiated logger of your own.
var DefaultLogger Logger = &logger{
	Logger: zap.NewNop(),
}

// NewProductionLogger is a reasonable production logging configuration.
// Logging is enabled at given level and above. The level can be later
// adjusted dynamically in runtime by calling SetLevel method.
//
// It uses the custom Key Value encoder, writes to standard error, and enables sampling.
// Stacktraces are automatically included on logs of ErrorLevel and above.
func NewProductionLogger(lvl *AtomicLevel, opts ...Option) Logger {
	opts = append(_defaultOption, opts...)

	var cfg logConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var zapOptions []zap.Option

	if cfg.caller {
		zapOptions = append(zapOptions, zap.AddCaller(), zap.AddCallerSkip(cfg.callerSkip))
	}

	if cfg.stacktrace {
		zapOptions = append(zapOptions, zap.AddStacktrace(zap.ErrorLevel))
	}

	zapOptions = append(zapOptions, wrapCoreWithLevel(lvl))

	l := zap.New(newZapCoreAtLevel(zap.DebugLevel, cfg), zapOptions...)

	return &logger{
		Logger: l,
	}
}

// logger provides a fast, leveled, structured logging. All methods are safe
// for concurrent use.
//
// The logger is designed for contexts in which every microsecond and every
// allocation matters, so its API intentionally favors performance and type
// safety over brevity. For most applications, the SugaredLogger strikes a
// better balance between performance and ergonomics.
type logger struct {
	*zap.Logger
}

var _ Logger = (*logger)(nil)

// WithLevel creates a child logger that logs on the given level.
// Child logger contains all fields from the parent.
func (l *logger) WithLevel(level Level) Logger {
	lvl := zap.NewAtomicLevelAt(level)
	child := l.Logger.WithOptions(wrapCoreWithLevel(&lvl))
	return &logger{
		Logger: child,
	}
}

// With creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func (l *logger) With(fields ...Field) Logger {
	child := l.Logger.With(fields...)
	return &logger{
		Logger: child,
	}
}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (l *logger) Named(s string) Logger {
	child := l.Logger.Named(s)
	return &logger{
		Logger: child,
	}
}

// Level reports the minimum enabled level for this logger.
func (l *logger) Level() Level {
	return zapcore.LevelOf(l.Core())
}

type WriteSyncer interface {
	io.Writer
	Sync() error
}

type encoderFactory func(config zapcore.EncoderConfig) zapcore.Encoder

type logConfig struct {
	levelKey   string
	caller     bool
	callerSkip int
	stacktrace bool
	writer     WriteSyncer

	encoderFactory encoderFactory
}

// Option configures a Logger.
type Option func(s *logConfig)

// WithLevelKey lets the caller configure which key name to use for the log level.
//
// Default value is "level".
func WithLevelKey(key string) Option {
	return func(s *logConfig) {
		s.levelKey = key
	}
}

// WithCaller lets the caller configure whether to include a "caller" tag in the
// log specifying the package/file:line in which the log occurred.
//
// Default value is "true", take into consideration that in order to obtain the
// caller value reflection is used, which has a runtime cost.
func WithCaller(t bool) Option {
	return func(s *logConfig) {
		s.caller = t
	}
}

// WithCallerSkip lets the caller configure whether to include a "caller skip" tag in the
// log specifying the number of callers skipped by caller annotation avoiding
// from always reporting the wrapper code as the caller.
//
// Default value is "1", take into consideration that in order to obtain the
// caller value reflection is used, which has a runtime cost.
func WithCallerSkip(skip int) Option {
	return func(s *logConfig) {
		s.callerSkip = skip
	}
}

// WithStacktraceOnError lets the caller configure whether to include a stacktrace
// on "Error" or higher log levels.
//
// Default value is "true", take into consideration that in order to obtain the
// stacktrace, reflection is used, which has a non-trivial runtime cost.
func WithStacktraceOnError(b bool) Option {
	return func(s *logConfig) {
		s.stacktrace = b
	}
}

// WithJSONEncoding tells the logger to use JSON as its encoding.
func WithJSONEncoding() Option {
	return func(s *logConfig) {
		s.encoderFactory = func(config zapcore.EncoderConfig) zapcore.Encoder {
			return zapcore.NewJSONEncoder(config)
		}
	}
}

// WithConsoleEncoding tells the logger to use a user-friendly console encoding as
// its encoding.
func WithConsoleEncoding() Option {
	return func(s *logConfig) {
		s.encoderFactory = func(config zapcore.EncoderConfig) zapcore.Encoder {
			return zapcore.NewConsoleEncoder(config)
		}
	}
}

// WithKeyValueEncoding tells the logger to use [key:value] as its encoding.
//
// This is the default setting.
func WithKeyValueEncoding(kveOption ...encoders.KeyValueEncoderOption) Option {
	return func(s *logConfig) {
		s.encoderFactory = func(config zapcore.EncoderConfig) zapcore.Encoder {
			return encoders.NewKeyValueEncoder(config, kveOption...)
		}
	}
}

// WithWriter lets the caller configure which WriteSyncer it wants the logger to
// write the logs to.
//
// Default value is to write to Stderr.
func WithWriter(w WriteSyncer) Option {
	return func(s *logConfig) {
		s.writer = w
	}
}

var (
	// Globally declare the stderr writer as we need to synchronize writes
	// between multiple instances of loggers.
	_stderr = zapcore.Lock(zapcore.AddSync(os.Stderr))

	// Default options used when constructing a logger.
	_defaultOption = []Option{
		WithWriter(_stderr),
		WithLevelKey("level"),
		WithStacktraceOnError(true),
		WithCaller(true),
		WithCallerSkip(1),
		WithKeyValueEncoding(),
	}
)

func newZapCoreAtLevel(lvl zapcore.Level, cfg logConfig) zapcore.Core {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       cfg.levelKey,
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     rfc3399NanoTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	return zapcore.NewCore(cfg.encoderFactory(encoderConfig), cfg.writer, lvl)
}

// rfc3399NanoTimeEncoder serializes a time.Time to an RFC3399-formatted string
// with microsecond precision padded with zeroes to make it fixed width.
func rfc3399NanoTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	const RFC3339Micro = "2006-01-02T15:04:05.000000Z07:00"

	enc.AppendString(t.UTC().Format(RFC3339Micro))
}
