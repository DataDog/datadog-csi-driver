// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

const modeSocket = "socket"

// socketLegacyPublisher handles the deprecated mode/path schema for socket mounts.
// This publisher is deprecated and will be removed in a future release.
// Use the "type" schema (e.g., type: APMSocket) instead.
type socketLegacyPublisher struct {
	fs            afero.Afero
	mounter       mount.Interface
	apmSocketPath string
	dsdSocketPath string
}

// Publish implements Publisher#Publish for the deprecated mode/path schema.
func (s socketLegacyPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()

	// Only handle legacy schema (mode/path without type)
	if _, hasType := volumeCtx["type"]; hasType {
		return nil, nil
	}

	mode, hasMode := volumeCtx["mode"]
	hostPath, hasPath := volumeCtx["path"]

	if !hasMode || !hasPath || mode != modeSocket {
		return nil, nil
	}

	klog.Warningf("Using deprecated mode/path schema. Please migrate to using 'type: APMSocket' or 'type: DSDSocket' instead.")

	resp := &PublisherResponse{VolumeType: VolumeType(mode), VolumePath: hostPath}
	targetPath := req.GetTargetPath()

	// Validate that hostPath is in the allowed list
	allowedPaths := []string{s.apmSocketPath, s.dsdSocketPath}
	if !isAllowedPath(hostPath, allowedPaths) {
		return resp, fmt.Errorf("path %q is not allowed; permitted paths are %v", hostPath, allowedPaths)
	}

	// Validate that hostPath is a socket
	hostPathIsSocket, err := isSocketPath(s.fs, hostPath)
	if err != nil {
		return resp, fmt.Errorf("failed to check if %q is a socket path: %w", hostPath, err)
	}
	if !hostPathIsSocket {
		return resp, fmt.Errorf("socket not found at %q", hostPath)
	}

	return resp, bindMount(s.fs, s.mounter, hostPath, targetPath, true)
}

func (s socketLegacyPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil // Handled by unmountPublisher
}

func newSocketLegacyPublisher(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath string) Publisher {
	return socketLegacyPublisher{fs: fs, mounter: mounter, apmSocketPath: apmSocketPath, dsdSocketPath: dsdSocketPath}
}
