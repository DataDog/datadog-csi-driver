// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

// PublisherResponse contains metadata about a handled request, used for metrics.
// A nil response means the publisher does not support the request.
type PublisherResponse struct {
	VolumeType VolumeType
	VolumePath string
}

// Publisher defines logic for publishing and unpublishing volumes.
// Each method returns:
//   - (*PublisherResponse, nil) if the operation succeeded
//   - (*PublisherResponse, error) if the operation failed
//   - (nil, nil) if the publisher does not support this request
type Publisher interface {
	// Publish publishes the volume
	Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error)
	// Unpublish unpublishes the volume
	Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error)
}

// GetPublishers returns a chain of publishers for handling CSI volume operations.
//
// The chain includes:
//   - Library publisher (for DatadogLibrary volumes)
//   - InjectorPreload publisher (for ld.so.preload injection)
//   - Socket/Local publishers (for "type" schema: APMSocket, APMSocketDirectory, etc.)
//   - Legacy publishers (for deprecated "mode/path" schema)
//   - Fallback unmount handler for all Unpublish requests
func GetPublishers(
	fs afero.Afero,
	mounter mount.Interface,
	apmSocketPath, dsdSocketPath, storageBasePath string,
	libraryManager *librarymanager.LibraryManager,
	apmEnabled bool,
) Publisher {
	// Order matters, the first publisher to return a response will stop the chain
	return newChainPublisher(
		// SSI publishers (library and injector preload)
		newLibraryPublisher(fs, mounter, libraryManager, !apmEnabled),
		newInjectorPreloadPublisher(fs, mounter, storageBasePath, !apmEnabled),

		// New "type" schema publishers
		newSocketPublisher(fs, mounter, apmSocketPath, dsdSocketPath),
		newLocalPublisher(fs, mounter, apmSocketPath, dsdSocketPath),

		// Legacy "mode/path" schema publishers (deprecated)
		newSocketLegacyPublisher(fs, mounter, apmSocketPath, dsdSocketPath),
		newLocalLegacyPublisher(fs, mounter, apmSocketPath, dsdSocketPath),

		// Fallback unmount handler for most Unpublish requests
		newUnmountPublisher(fs, mounter),
	)
}
