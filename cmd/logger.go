package main

import (
	log "log/slog"
	"os"
)

func init() {
	// Configure slog for JSON output
	// Default to Info level, can be overridden via environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	var level log.Level
	switch logLevel {
	case "debug":
		level = log.LevelDebug
	case "warn":
		level = log.LevelWarn
	case "error":
		level = log.LevelError
	default:
		level = log.LevelInfo
	}

	// Create JSON handler with structured output to stdout
	handler := log.NewJSONHandler(os.Stdout, &log.HandlerOptions{
		Level: level,
	})

	// Create logger with service name attribute
	logger := log.New(handler).With("service", "datadog-csi-driver")

	// Set as default logger
	log.SetDefault(logger)
}
