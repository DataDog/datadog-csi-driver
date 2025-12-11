// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

const modeLocal = "local"

// localLegacyPublisher handles the deprecated mode/path schema for local (directory) mounts.
// This publisher is deprecated and will be removed in a future release.
// Use the "type" schema (e.g., type: APMSocketDirectory) instead.
type localLegacyPublisher struct {
	fs            afero.Afero
	mounter       mount.Interface
	apmSocketPath string
	dsdSocketPath string
}

func (s localLegacyPublisher) Stage(req *csi.NodeStageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s localLegacyPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

// Publish implements Publisher#Publish for the deprecated mode/path schema.
func (s localLegacyPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()

	// Only handle legacy schema (mode/path without type)
	if _, hasType := volumeCtx["type"]; hasType {
		return nil, nil
	}

	mode, hasMode := volumeCtx["mode"]
	hostPath, hasPath := volumeCtx["path"]

	if !hasMode || !hasPath || mode != modeLocal {
		return nil, nil
	}

	klog.Warningf("Using deprecated mode/path schema. Please migrate to using 'type: APMSocketDirectory' or 'type: DSDSocketDirectory' instead.")

	resp := &PublisherResponse{VolumeType: VolumeType(mode), VolumePath: hostPath}
	targetPath := req.GetTargetPath()

	// Validate that hostPath is in the allowed list (parent directories of the sockets)
	allowedPaths := []string{filepath.Dir(s.apmSocketPath), filepath.Dir(s.dsdSocketPath)}
	if !isAllowedPath(hostPath, allowedPaths) {
		return resp, fmt.Errorf("path %q is not allowed; permitted paths are %v", hostPath, allowedPaths)
	}

	return resp, bindMount(s.fs, s.mounter, hostPath, targetPath, false)
}

func (s localLegacyPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil // Handled by unmountPublisher
}

func newLocalLegacyPublisher(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath string) Publisher {
	return localLegacyPublisher{fs: fs, mounter: mounter, apmSocketPath: apmSocketPath, dsdSocketPath: dsdSocketPath}
}
