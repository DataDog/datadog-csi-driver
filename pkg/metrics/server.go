// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// MetricsServer is the metrics server interface
type MetricsServer interface {
	// Start runs the metrics server.
	// This should be a blocking call until the context is cancelled or done.
	Start(ctx context.Context, errChan chan error)
}

// server implements MetricsServer
type server struct {
	srv *http.Server
}

// Start implements MetricsServer#Start
func (s *server) Start(ctx context.Context, errChan chan error) {
	log.Info().Msg("starting metrics server")
	// Run server
	go func() {
		err := s.srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("error starting metrics server: %v", err)
		}
	}()

	// Shutdown server when context is done
	<-ctx.Done()
	if err := s.close(); err != nil {
		log.Error().Err(err).Msg("error closing metrics server")
	}
}

// close closes the http server
func (s *server) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		log.Warn().Err(err).Msg("Problem shutting down metrics HTTP server")
		return err
	}
	return nil
}

// buildServer creates the http.Server struct
func buildServer(port int) (*server, error) {

	if port <= 0 {
		log.Error().Msg("Invalid port for metric server")
		return nil, errors.New("invalid port for metrics server")
	}

	bindAddr := fmt.Sprintf(":%d", port)
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:    bindAddr,
		Handler: router,
	}

	return &server{srv}, nil
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(port int) (MetricsServer, error) {
	return buildServer(port)
}
