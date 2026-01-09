// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

// registerAndStartCSIDriver registers the CSI driver and starts it
// This is a blocking operation.
func registerAndStartCSIDriver(ctx context.Context) error {
	// Create CSI driver

	flag.Parse()

	csiDriver, err := driver.NewDatadogCSIDriver(
		*driverNameFlag,
		*apmHostSocketPath,
		*dsdHostSocketPath,
		Version,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create CSI driver")
		return err
	}

	// Log the version
	log.Info().Str("version", csiDriver.Version()).Msg("Created Datadog CSI Driver")

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
	log.Info().Msg("starting GRPC server for CSI driver")
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
