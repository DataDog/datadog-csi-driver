// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"context"
	log "log/slog"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func (d *DatadogCSIDriver) GetPluginInfo(_ context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.Debug("Method GetPluginInfo called", "request", req)

	return &csi.GetPluginInfoResponse{
		Name:          d.name,
		VendorVersion: d.version,
	}, nil
}

func (d *DatadogCSIDriver) GetPluginCapabilities(_ context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.Debug("Method GetPluginCapabilities called", "request", req)

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{},
				},
			},
		},
	}, nil
}

func (d *DatadogCSIDriver) Probe(_ context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	log.Debug("Method Probe called", "request", req)

	return &csi.ProbeResponse{}, nil
}
