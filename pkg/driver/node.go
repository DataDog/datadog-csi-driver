// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog"
)

func (d *DatadogCSIDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *DatadogCSIDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: os.Getenv("NODE_ID"), // this is a unique identifier of the node
	}, nil
}

func (d *DatadogCSIDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof(
		"Received NodeStageVolumeRequest with volume id = %v, staging target path = %v",
		req.GetVolumeId(),
		req.GetStagingTargetPath(),
	)

	supported, err := d.publisher.Stage(req)
	if err != nil {
		return nil, fmt.Errorf("failed to stage volume: %v", err)
	}

	// Not all publishers support staging, so we don't return an error
	if !supported {
		klog.Infof("stage volume request not supported by any publisher")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *DatadogCSIDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.Infof(
		"Received NodeUnstageVolumeRequest with volume id = %v, staging target path = %v",
		req.GetVolumeId(),
		req.GetStagingTargetPath(),
	)

	supported, err := d.publisher.Unstage(req)
	if err != nil {
		return nil, fmt.Errorf("failed to unstage volume: %v", err)
	}

	// Not all publishers support staging, so we don't return an error
	if !supported {
		klog.Infof("unstage volume request not supported by any publisher")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *DatadogCSIDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof(
		"Received NodePublishVolumeRequest with target path = %v, volume id = %v, volume context = %v",
		req.GetTargetPath(),
		req.GetVolumeId(),
		req.GetVolumeContext(),
	)

	supported, err := d.publisher.Publish(req)
	if err != nil {
		return nil, fmt.Errorf("failed to publish volume: %v", err)
	}
	if !supported {
		return nil, fmt.Errorf("publish volume request not supported by any publisher")
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *DatadogCSIDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof(
		"Received NodeUnpublishVolumeRequest with target path = %v, volume id = %v",
		req.GetTargetPath(),
		req.GetVolumeId(),
	)

	supported, err := d.publisher.Unpublish(req)
	if err != nil {
		return nil, fmt.Errorf("failed to unpublish volume: %v", err)
	}
	if !supported {
		return nil, fmt.Errorf("unpublish volume request not supported by any publisher")
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}
