// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

// socketPublisher handles socket mounts using the "type" schema (APMSocket, DSDSocket, DSDStreamSocket).
type socketPublisher struct {
	fs                  afero.Afero
	mounter             mount.Interface
	apmSocketPath       string
	dsdSocketPath       string
	dsdStreamSocketPath string
}

// Publish implements Publisher#Publish for the "type" schema.
// It handles APMSocket, DSDSocket, and DSDStreamSocket volume types.
func (s socketPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()

	// Resolve the type to hostPath
	var hostPath string
	volumeType := VolumeType(volumeCtx["type"])
	switch volumeType {
	case APMSocket:
		hostPath = s.apmSocketPath
	case DSDSocket:
		hostPath = s.dsdSocketPath
	case DSDStreamSocket:
		hostPath = s.dsdStreamSocketPath
	default:
		return nil, nil
	}

	resp := &PublisherResponse{VolumeType: volumeType, VolumePath: hostPath}
	targetPath := req.GetTargetPath()

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

func (s socketPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil // Handled by unmountPublisher
}

func newSocketPublisher(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath, dsdStreamSocketPath string) Publisher {
	return socketPublisher{fs: fs, mounter: mounter, apmSocketPath: apmSocketPath, dsdSocketPath: dsdSocketPath, dsdStreamSocketPath: dsdStreamSocketPath}
}
