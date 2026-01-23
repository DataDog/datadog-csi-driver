// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package main

import (
	"context"
	"fmt"
	log "log/slog"
	"os"
	"strings"
	"sync"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/pkg/metrics"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Version = "dev" // This will be set when building the driver

func init() {
	// Define flags
	pflag.String("driver-name", driver.CSIDriverName, "Name of the CSI driver")
	pflag.String("csi-endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	pflag.String("dsd-host-socket-path", "/var/run/datadog/dsd.socket", "Dogstatsd socket host path")
	pflag.String("apm-host-socket-path", "/var/run/datadog/apm.socket", "APM socket host path")
	pflag.String("storage-path", "/var/lib/datadog-csi-driver", "Base path for CSI driver storage")

	// Disable SSI publishers (library and injector preload)
	// Publish requests will be rejected if SSI is disabled, but Unpublish requests will still be handled.
	pflag.Bool("disable-ssi", false, "Disable SSI publishers (library and injector preload)")

	// Parse flags
	pflag.Parse()

	// Bind flags to viper
	viper.BindPFlags(pflag.CommandLine)

	// Configure env var support
	// DD_DRIVER_NAME, DD_CSI_ENDPOINT, DD_DSD_HOST_SOCKET_PATH, etc.
	viper.SetEnvPrefix("DD")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

// run creates and runs the metrics server and the csi driver grpc server
// It can only return a non-nil error
// It provides no guarantee on the order by which the servers are started.
// It guarantees that if an error occurs in one server, both servers are shutdown.
func run() error {
	// wait group used to ensure that all go routines terminate before returning an error from this function
	wg := &sync.WaitGroup{}

	errChan := make(chan error)
	defer close(errChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create metrics server
	metricsServer, err := metrics.NewMetricsServer(metrics.MetricsPort)
	if err != nil {
		return fmt.Errorf("failed to create metrics server: %v", err)
	}

	// Start metrics server
	wg.Add(1)
	go func() {
		metricsServer.Start(ctx, errChan)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := registerAndStartCSIDriver(ctx); err != nil {
			errChan <- fmt.Errorf("failed starting csi driver: %v", err)
		}
		wg.Done()
	}()

	err = <-errChan
	cancel() // cancelling the context allows stopping both the grpc and the metrics server in case of error
	log.Info("Waiting for servers to stop gracefully.")
	wg.Wait() // block until all goroutines have finished
	log.Info("Graceful stop finished.")
	return err
}

func main() {
	if err := run(); err != nil {
		log.Error("Fatal error", "error", err)
		os.Exit(1)
	}
}
