// Package logger wraps charmbracelet/log behind slog, providing leveled
// structured logging with optional color and timestamps.
package logger

import (
	"log/slog"
	"os"
	"strings"

	charmlog "github.com/charmbracelet/log"
	"github.com/muesli/termenv"
)

var defaultLogger *slog.Logger

// Initialize sets the global log level and output format.
func Initialize(logLevel string, plain bool) {
	var level charmlog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = charmlog.DebugLevel
	case "warn", "warning":
		level = charmlog.WarnLevel
	case "error":
		level = charmlog.ErrorLevel
	default:
		level = charmlog.InfoLevel
	}

	opts := charmlog.Options{
		Level:           level,
		ReportTimestamp: plain,
	}
	logger := charmlog.NewWithOptions(os.Stderr, opts)
	if plain {
		logger.SetFormatter(charmlog.TextFormatter)
		logger.SetStyles(charmlog.DefaultStyles()) // reset to avoid nil
		// Force no-color output for plain/server mode
		logger.SetColorProfile(termenv.Ascii)
	}

	defaultLogger = slog.New(logger)
}

// Info logs a message at INFO level with optional structured key-value pairs.
func Info(msg string, keysAndValues ...interface{}) {
	defaultLogger.Info(msg, keysAndValues...)
}

// Debug logs a message at DEBUG level; suppressed unless verbose logging is enabled.
func Debug(msg string, keysAndValues ...interface{}) {
	defaultLogger.Debug(msg, keysAndValues...)
}

// Warn logs a message at WARN level.
func Warn(msg string, keysAndValues ...interface{}) {
	defaultLogger.Warn(msg, keysAndValues...)
}

// Error logs a message at ERROR level.
func Error(msg string, keysAndValues ...interface{}) {
	defaultLogger.Error(msg, keysAndValues...)
}

func init() {
	Initialize("info", false)
}
