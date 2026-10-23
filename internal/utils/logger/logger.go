package logger

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger
var sugar *zap.SugaredLogger

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	if log == nil {
		// Initialize with default config if not initialized
		Init("info")
	}
	return log
}

// Options configure the global logger.
type Options struct {
	// Color enables ANSI level colouring. Callers decide this, because only they
	// know whether the destination is a terminal and whether NO_COLOR is set
	// (CLI-7). The zero value is off, which is the safe default.
	Color bool
	// JSON forces structured JSON output. When unset, JSON is used unless Color is
	// on, so an operator at a terminal gets readable console output and anything
	// collecting logs gets parseable ones (OBS-1).
	JSON bool
	// Service and Version are attached to every line (OBS-1).
	Service string
	Version string
}

// Option modifies Options.
type Option func(*Options)

// WithColor enables or disables ANSI colour.
func WithColor(enabled bool) Option { return func(o *Options) { o.Color = enabled } }

// WithJSON forces JSON encoding regardless of whether colour is enabled.
func WithJSON(enabled bool) Option { return func(o *Options) { o.JSON = enabled } }

// WithService sets the service and version fields carried on every line.
func WithService(service, version string) Option {
	return func(o *Options) { o.Service, o.Version = service, version }
}

// Init initializes the logger with the specified level.
//
// Logs always go to stderr: stdout carries results only (CLI-5).
func Init(level string, opts ...Option) error {
	zapLevel, err := parseLevel(level)
	if err != nil {
		return err
	}

	options := Options{Service: "flowctl"}
	for _, opt := range opts {
		opt(&options)
	}

	encodeLevel := zapcore.CapitalLevelEncoder
	encoding := "json"
	switch {
	case options.JSON:
		// explicit JSON wins
	case options.Color:
		encoding = "console"
		encodeLevel = zapcore.CapitalColorLevelEncoder
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Encoding:    encoding,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    encodeLevel,
			EncodeTime:     utcRFC3339,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	fields := []zap.Field{zap.String("service", options.Service)}
	if options.Version != "" {
		fields = append(fields, zap.String("version", options.Version))
	}
	logger = logger.With(fields...)

	log = logger
	sugar = logger.Sugar()
	return nil
}

// utcRFC3339 encodes timestamps as RFC 3339 in UTC with a Z suffix, as DATA-3
// requires. zap's own RFC3339 encoders emit a local offset instead.
func utcRFC3339(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format(time.RFC3339Nano))
}

// parseLevel converts a string log level to a zapcore.Level
func parseLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// Debug logs a message at debug level
func Debug(msg string, fields ...zap.Field) {
	if log != nil {
		log.Debug(msg, fields...)
	}
}

// Info logs a message at info level
func Info(msg string, fields ...zap.Field) {
	if log != nil {
		log.Info(msg, fields...)
	}
}

// Warn logs a message at warn level
func Warn(msg string, fields ...zap.Field) {
	if log != nil {
		log.Warn(msg, fields...)
	}
}

// Error logs a message at error level
func Error(msg string, fields ...zap.Field) {
	if log != nil {
		log.Error(msg, fields...)
	}
}

// Fatal logs a message at fatal level and then calls os.Exit(1)
func Fatal(msg string, fields ...zap.Field) {
	if log != nil {
		log.Fatal(msg, fields...)
	}
}

// Debugf logs a formatted message at debug level
func Debugf(template string, args ...interface{}) {
	if sugar != nil {
		sugar.Debugf(template, args...)
	}
}

// Infof logs a formatted message at info level
func Infof(template string, args ...interface{}) {
	if sugar != nil {
		sugar.Infof(template, args...)
	}
}

// Warnf logs a formatted message at warn level
func Warnf(template string, args ...interface{}) {
	if sugar != nil {
		sugar.Warnf(template, args...)
	}
}

// Errorf logs a formatted message at error level
func Errorf(template string, args ...interface{}) {
	if sugar != nil {
		sugar.Errorf(template, args...)
	}
}

// Fatalf logs a formatted message at fatal level and then calls os.Exit(1)
func Fatalf(template string, args ...interface{}) {
	if sugar != nil {
		sugar.Fatalf(template, args...)
	}
}

// With creates a child logger and adds structured context to it
func With(fields ...zap.Field) *zap.Logger {
	if log != nil {
		return log.With(fields...)
	}
	return nil
}

// WithFields creates a child sugared logger with structured context
func WithFields(fields map[string]interface{}) *zap.SugaredLogger {
	if sugar != nil {
		return sugar.With(fieldsToArgs(fields)...)
	}
	return nil
}

// fieldsToArgs converts a map to a list of alternating key/value pairs
func fieldsToArgs(fields map[string]interface{}) []interface{} {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return args
}

// Sync flushes any buffered log entries
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}
