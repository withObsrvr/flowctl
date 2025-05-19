package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger
var sugar *zap.SugaredLogger

// Init initializes the logger with the specified level
func Init(level string) error {
	zapLevel, err := parseLevel(level)
	if err != nil {
		return err
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapLevel),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
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

	log = logger
	sugar = logger.Sugar()
	return nil
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