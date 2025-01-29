package main

import (
	"flag"
	"net"
	"os"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

var (
	driverNameFlag = flag.String("driver-name", driver.CSIDriverName, "Name of the CSI driver")
	endpointFlag   = flag.String("csi-endpoint", "unix:///csi/csi.sock", "CSI endpoint")
)

func main() {

	// Create CSI driver
	csiDriver, err := driver.NewDatadogCSIDriver(*driverNameFlag)
	if err != nil {
		klog.Error(err.Error())
		os.Exit(1)
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
		klog.Fatalf("Failed to listen on endpoint %q: %v", endpoint, err)
		os.Exit(1)
	}

	listener, err := net.Listen("unix", unixAddress)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	// Start server
	klog.Info("Starting GRPC server for CSI driver")
	if err := grpcServer.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve: %v", err)
	}

}
