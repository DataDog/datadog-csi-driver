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

const modeSocket = "socket"

// socketPublisher handles socket mounts using the "type" schema (APMSocket, DSDSocket).
type socketPublisher struct {
	fs            afero.Afero
	mounter       mount.Interface
	apmSocketPath string
	dsdSocketPath string
}

func (s socketPublisher) Stage(req *csi.NodeStageVolumeRequest) (bool, error) {
	return false, nil
}

func (s socketPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (bool, error) {
	return false, nil
}

// Publish implements Publisher#Publish for the "type" schema.
// It handles APMSocket and DSDSocket volume types.
func (s socketPublisher) Publish(req *csi.NodePublishVolumeRequest) (bool, error) {
	volumeCtx := req.GetVolumeContext()

	// Resolve the type to hostPath
	var hostPath string
	switch VolumeType(volumeCtx["type"]) {
	case APMSocket:
		hostPath = s.apmSocketPath
	case DSDSocket:
		hostPath = s.dsdSocketPath
	default:
		return false, nil
	}

	targetPath := req.GetTargetPath()

	// Validate that hostPath is a socket
	hostPathIsSocket, err := isSocketPath(hostPath)
	if err != nil {
		return true, fmt.Errorf("failed to check if %q is a socket path: %w", hostPath, err)
	}
	if !hostPathIsSocket {
		return true, fmt.Errorf("socket not found at %q", hostPath)
	}

	return true, bindMount(s.fs, s.mounter, hostPath, targetPath, true)
}

func (s socketPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (bool, error) {
	return false, nil // Handled by unmountPublisher
}

func newSocketPublisher(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath string) Publisher {
	return socketPublisher{fs: fs, mounter: mounter, apmSocketPath: apmSocketPath, dsdSocketPath: dsdSocketPath}
}
