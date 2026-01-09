// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

// localPublisher handles directory mounts using the "type" schema
// (APMSocketDirectory, DSDSocketDirectory, DatadogSocketsDirectory).
type localPublisher struct {
	fs            afero.Afero
	mounter       mount.Interface
	apmSocketPath string
	dsdSocketPath string
}

// Publish implements Publisher#Publish for the "type" schema.
// It handles APMSocketDirectory, DSDSocketDirectory, and DatadogSocketsDirectory volume types.
func (s localPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()

	// Resolve the type to hostPath (parent directory of the socket)
	var hostPath string
	volumeType := VolumeType(volumeCtx["type"])
	switch volumeType {
	case APMSocketDirectory:
		hostPath = filepath.Dir(s.apmSocketPath)
	case DSDSocketDirectory:
		hostPath = filepath.Dir(s.dsdSocketPath)
	case DatadogSocketsDirectory:
		klog.Warningf("%s volume type is deprecated. Prefer using %s or %s instead.",
			DatadogSocketsDirectory, DSDSocketDirectory, APMSocketDirectory)
		hostPath = filepath.Dir(s.dsdSocketPath)
	default:
		return nil, nil
	}

	resp := &PublisherResponse{VolumeType: volumeType, VolumePath: hostPath}
	targetPath := req.GetTargetPath()

	return resp, bindMount(s.fs, s.mounter, hostPath, targetPath, false)
}

func (s localPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil // Handled by unmountPublisher
}

func newLocalPublisher(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath string) Publisher {
	return localPublisher{fs: fs, mounter: mounter, apmSocketPath: apmSocketPath, dsdSocketPath: dsdSocketPath}
}
