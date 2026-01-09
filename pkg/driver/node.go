// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"context"
	"fmt"
	"os"

	"github.com/Datadog/datadog-csi-driver/pkg/metrics"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog"
)

func (d *DatadogCSIDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (d *DatadogCSIDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: os.Getenv("NODE_ID"), // this is a unique identifier of the node
	}, nil
}

func (d *DatadogCSIDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof(
		"Received NodePublishVolumeRequest with target path = %v, volume id = %v, volume context = %v",
		req.GetTargetPath(),
		req.GetVolumeId(),
		req.GetVolumeContext(),
	)

	resp, err := d.publisher.Publish(req)
	if err != nil {
		metrics.RecordVolumeMountAttempt(string(resp.VolumeType), resp.VolumePath, metrics.StatusFailed)
		return nil, fmt.Errorf("failed to publish volume: %v", err)
	}

	// Not all publishers support all volume types, so we don't return an error if resp is nil
	if resp == nil {
		klog.Warningf("publish volume request not supported by any publisher")
		volumeCtx := req.GetVolumeContext()
		metrics.RecordVolumeMountAttempt(volumeCtx["type"], req.GetTargetPath(), metrics.StatusUnsupported)
	} else {
		metrics.RecordVolumeMountAttempt(string(resp.VolumeType), resp.VolumePath, metrics.StatusSuccess)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *DatadogCSIDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof(
		"Received NodeUnpublishVolumeRequest with target path = %v, volume id = %v",
		req.GetTargetPath(),
		req.GetVolumeId(),
	)

	resp, err := d.publisher.Unpublish(req)
	if err != nil {
		metrics.RecordVolumeUnMountAttempt(metrics.StatusFailed)
		return nil, fmt.Errorf("failed to unpublish volume: %v", err)
	}

	// Not all publishers support all volume types, so we don't return an error if resp is nil
	if resp == nil {
		klog.Warningf("unpublish volume request not supported by any publisher")
		metrics.RecordVolumeUnMountAttempt(metrics.StatusUnsupported)
	} else {
		metrics.RecordVolumeUnMountAttempt(metrics.StatusSuccess)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}
