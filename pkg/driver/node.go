package driver

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"

	"github.com/Datadog/datadog-csi-driver/pkg/driver/publishers"
	"github.com/Datadog/datadog-csi-driver/pkg/metrics"
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
	targetPath := req.GetTargetPath()
	volumeId := req.GetVolumeId()
	volumeCtx := req.GetVolumeContext()

	klog.Infof(
		"Received NodePublishVolumeRequest with target path = %v, volume id = %v, volume context = %v",
		targetPath,
		volumeId,
		volumeCtx,
	)

	ddVolumeRequest, err := NewDDVolumeRequest(req)
	if err != nil {
		metrics.RecordVolumeMountAttempt("", "", metrics.StatusFailed)
		return nil, fmt.Errorf("failed to create dd volume request: %v", err)
	}

	publisher, found := d.publishers[publishers.PublisherKind(ddVolumeRequest.mode)]
	if !found {
		metrics.RecordVolumeMountAttempt(ddVolumeRequest.mode, ddVolumeRequest.path, metrics.StatusFailed)
		return nil, fmt.Errorf("invalid mode: %q", ddVolumeRequest.mode)
	}

	err = publisher.Mount(ddVolumeRequest.targetpath, ddVolumeRequest.path)
	if err != nil {
		metrics.RecordVolumeMountAttempt(ddVolumeRequest.mode, ddVolumeRequest.path, metrics.StatusFailed)
		return nil, fmt.Errorf("failed to perform volume mount: %v", err)
	}

	metrics.RecordVolumeMountAttempt(ddVolumeRequest.mode, ddVolumeRequest.path, metrics.StatusSuccess)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *DatadogCSIDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof(
		"Received NodeUnPublishVolumeRequest with target path = %v, volume id = %v",
		req.GetTargetPath(),
		req.GetVolumeId(),
	)

	targetPath := req.GetTargetPath()

	// Check if the target path is a mount point. If it's not a mount point, nothing needs to be done.
	isNotMnt, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the target path doesn't exist, there's nothing to unmount,
			// but we return success because from a CSI perspective, the volume is no longer published.
			klog.Info("Target path does not exist, nothing to unmount.")
			metrics.RecordVolumeUnMountAttempt(metrics.StatusSuccess)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}

		metrics.RecordVolumeUnMountAttempt(metrics.StatusFailed)
		klog.Infof("Failed to check if target path is a mount point: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to check if target path is a mount point: %v", err)
	}

	// If it's a mount point, proceed to unmount
	if isNotMnt {
		klog.Infof("Target path %s is not a mount point, skipping.", targetPath)
	} else {
		// Unmount the target path
		if err := d.mounter.Unmount(targetPath); err != nil {
			metrics.RecordVolumeUnMountAttempt(metrics.StatusFailed)
			klog.Infof("failed to unmount target path %q: %v", targetPath, err)
			return nil, status.Errorf(codes.Internal, "failed to unmount target path %q: %v", targetPath, err)
		}
	}

	// After unmounting, you may also want to remove the directory to clean up, depending on your use case.
	if err := os.RemoveAll(targetPath); err != nil {
		metrics.RecordVolumeUnMountAttempt(metrics.StatusFailed)
		return nil, status.Errorf(codes.Internal, "failed to remove target path %s: %v", targetPath, err)
	}

	metrics.RecordVolumeUnMountAttempt(metrics.StatusSuccess)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}
