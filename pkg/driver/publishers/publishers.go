// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

// Publisher defines logic for staging, unstaging, publishing and unpublishing volumes.
// A publisher returns true if it supports the operation for the given request, false otherwise.
type Publisher interface {
	// Stage stages the volume
	Stage(req *csi.NodeStageVolumeRequest) (bool, error)
	// Unstage unstages the volume
	Unstage(req *csi.NodeUnstageVolumeRequest) (bool, error)
	// Publish publishes the volume
	Publish(req *csi.NodePublishVolumeRequest) (bool, error)
	// Unpublish unpublishes the volume
	Unpublish(req *csi.NodeUnpublishVolumeRequest) (bool, error)
}

// GetPublishers returns a chain of publishers for handling CSI volume operations.
// The apmSocketPath and dsdSocketPath are the paths to the Datadog agent sockets.
//
// The chain includes:
//   - SSI publisher (for Single Step Instrumentation)
//   - Socket/Local publishers (for "type" schema: APMSocket, APMSocketDirectory, etc.)
//   - Legacy publishers (for deprecated "mode/path" schema)
//   - Fallback unmount handler for all Unpublish requests
func GetPublishers(fs afero.Afero, mounter mount.Interface, apmSocketPath, dsdSocketPath string) Publisher {
	return newChainPublisher(
		// Order matters, the first publisher to return true will stop the chain
		newSSIPublisher(fs, mounter),

		// New "type" schema publishers
		newSocketPublisher(fs, mounter, apmSocketPath, dsdSocketPath),
		newLocalPublisher(fs, mounter, apmSocketPath, dsdSocketPath),

		// Legacy "mode/path" schema publishers (deprecated)
		newSocketLegacyPublisher(fs, mounter, apmSocketPath, dsdSocketPath),
		newLocalLegacyPublisher(fs, mounter, apmSocketPath, dsdSocketPath),

		// Fallback unmount handler for all Unpublish requests
		newUnmountPublisher(mounter),
	)
}
