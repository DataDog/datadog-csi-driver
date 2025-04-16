package main

import (
	"context"
	"fmt"
	"net"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

// registerAndStartCSIDriver registers the CSI driver and starts it
// This is a blocking operation.
func registerAndStartCSIDriver(ctx context.Context) error {
	// Create CSI driver

	csiDriver, err := driver.NewDatadogCSIDriver(
		*driverNameFlag,
		*apmHostSocketPath,
		*dsdHostSocketPath,
		Version,
	)
	if err != nil {
		klog.Error(err.Error())
		return err
	}

	// Log the version
	klog.Infof("Created Datadog CSI Driver version %v", csiDriver.Version())

	// Setup grpc server
	// TODO: check if it is necessary to use TLS in the grpc server
	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, csiDriver)
	csi.RegisterNodeServer(grpcServer, csiDriver)

	// Define unix socket listener
	endpoint := *endpointFlag
	unixAddress, err := utils.EnsureSocketAvailability(endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen on endpoint %q: %v", endpoint, err)
	}

	listener, err := net.Listen("unix", unixAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	errChan := make(chan error, 1)
	defer close(errChan)

	// Starting the GRPC server for CSI
	klog.Info("starting GRPC server for CSI driver")
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			errChan <- fmt.Errorf("csi grpc failed to serve: %v", err)
		}
	}()
	defer grpcServer.GracefulStop()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return nil
	}
}
