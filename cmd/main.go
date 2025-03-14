package main

import (
	"context"
	"flag"

	"k8s.io/klog"

	"github.com/Datadog/datadog-csi-driver/pkg/driver"
	"github.com/Datadog/datadog-csi-driver/pkg/metrics"
)

var (
	driverNameFlag = flag.String("driver-name", driver.CSIDriverName, "Name of the CSI driver")
	endpointFlag   = flag.String("csi-endpoint", "unix:///csi/csi.sock", "CSI endpoint")
)

func main() {
	metricsServer, err := metrics.NewMetricsServer(metrics.MetricsPort)
	if err != nil {
		klog.Fatalf("failed to create metrics server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go metricsServer.Start(ctx)
	defer cancel()

	if err := registerAndStartCSIDriver(ctx); err != nil {
		klog.Fatal(err)
	}
}
