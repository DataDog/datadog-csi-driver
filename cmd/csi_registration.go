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

func registerAndStartCSIDriver(ctx context.Context) error {
	// Create CSI driver
	csiDriver, err := driver.NewDatadogCSIDriver(*driverNameFlag)
	if err != nil {
		klog.Error(err.Error())
		return err
	}

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

	// Starting the GRPC server for CSI
	errChan := make(chan error, 1)

	klog.Info("starting GRPC server for CSI driver")
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			klog.Errorf("failed to serve: %v", err)
		}
	}()

	// Listen for context cancellation to stop the server
	go func() {
		<-ctx.Done()
		klog.Info("stopping GRPC server for CSI driver")
		grpcServer.GracefulStop()
	}()

	// Wait for either an error or context cancellation
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return nil
	}
}
