package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
)

// DatadogCSIDriver is datadog CSI driver implementing CSI Node and Identity Server
type DatadogCSIDriver struct {
	csi.UnimplementedNodeServer
	csi.UnimplementedIdentityServer
	name    string
	version string
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name string) (*DatadogCSIDriver, error) {
	return &DatadogCSIDriver{
		name: name,

		// hardcoded for now
		// TODO: allow setting version dynamically on build
		version: "v1",
	}, nil
}
