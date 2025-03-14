package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

// MetricsServer is the metrics server interface
type MetricsServer interface {
	// Start runs the metrics server.
	// This should be a blocking call until the context is cancelled or done.
	Start(ctx context.Context)
}

// server implements MetricsServer
type server struct {
	srv *http.Server
}

// Start implements MetricsServer#Start
func (s *server) Start(ctx context.Context) {
	klog.Info("starting metrics server")
	// Run server
	go func() {
		err := s.srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			klog.Errorf("error starting metrics server: %v", err)
		}
	}()

	// Shutdown server when context is done
	<-ctx.Done()
	if err := s.close(); err != nil {
		klog.Errorf("error closing metrics server: %v", err)
	}
}

// close closes the http server
func (s *server) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		klog.Warningf("Problem shutting down metrics HTTP server: %v", err)
		return err
	}
	return nil
}

// buildServer creates the http.Server struct
func buildServer(port int) (*server, error) {

	if port <= 0 {
		klog.Error("invalid port for metric server")
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
