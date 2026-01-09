package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// Configure zerolog for JSON output (optimal for Datadog log collection)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Create logger with structured output to stdout
	// Datadog will parse JSON logs and properly categorize severity levels
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "datadog-csi-driver").
		Logger()

	// Set the global logger
	log.Logger = logger

	// Default to Info level, can be overridden via environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}
