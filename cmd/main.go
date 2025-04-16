package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/pkg/metrics"
	"k8s.io/klog/v2"
)

var (
	Version           = "dev" // This will be set when building the driver
	driverNameFlag    = flag.String("driver-name", driver.CSIDriverName, "Name of the CSI driver")
	endpointFlag      = flag.String("csi-endpoint", "unix:///csi/csi.sock", "CSI endpoint")
	dsdHostSocketPath = flag.String("dsd-host-socket-path", "/var/run/datadog/dsd.socket", "Dogstatsd socket host path")
	apmHostSocketPath = flag.String("apm-host-socket-path", "/var/run/datadog/apm.socket", "APM socket host path")
)

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
	klog.Info("Waiting for servers to stop gracefully.")
	wg.Wait() // block until all goroutines have finished
	klog.Info("Gracefull stop finished.")
	return err
}

func main() {
	if err := run(); err != nil {
		klog.Error(err)
		klog.Flush()
		os.Exit(1)
	}
}
