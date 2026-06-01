// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package main

import (
	log "log/slog"
	"os"
)

func init() {
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

	handler := log.NewJSONHandler(os.Stdout, &log.HandlerOptions{
		Level: level,
	})
	log.SetDefault(log.New(handler))
}
