// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

const (
	// VolumeContext keys for DatadogLibrary volumes
	keyLibraryPackage  = "dd.csi.datadog.com/library.package"
	keyLibraryRegistry = "dd.csi.datadog.com/library.registry"
	keyLibraryVersion  = "dd.csi.datadog.com/library.version"

	// Ssource path inside the OCI images
	languageLibrarySourcePath = "/datadog-init/package"
	injectorLibrarySourcePath = "/opt/datadog-packages/datadog-apm-inject"
)

// libraryPublisher handles DatadogLibrary volumes.
// It downloads OCI images containing instrumentation libraries and mounts them.
type libraryPublisher struct {
	fs             afero.Afero
	mounter        mount.Interface
	libraryManager *librarymanager.LibraryManager
}

// Publish downloads the library from the OCI registry if needed and bind-mounts it to the target path.
func (s libraryPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()
	if VolumeType(volumeCtx["type"]) != DatadogLibrary {
		return nil, nil // Not our volume
	}

	// Defensive code: library volumes must be mounted in read-only mode to protect the shared store
	if !req.GetReadonly() {
		return &PublisherResponse{VolumeType: DatadogLibrary}, fmt.Errorf("library volumes must be mounted in read-only mode")
	}

	libraryPath, image, err := s.getLibraryPath(volumeCtx, req.GetVolumeId())
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: image}, err
	}

	err = bindMount(s.fs, s.mounter, libraryPath, req.GetTargetPath(), false)
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: image}, err
	}

	return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: image}, nil
}

// Unpublish unmounts the library from the target path.
// For inline CSI volumes, Kubernetes doesn't call Unstage, so we also remove the volume
// tracking here to ensure libraries are cleaned up when no longer used.
func (s libraryPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	// We don't have VolumeContext in Unpublish, so we check if the volume is managed by us
	volumeID := req.GetVolumeId()

	// Check if this volume is managed by the library manager
	hasVolume, err := s.libraryManager.HasVolume(volumeID)
	if err != nil {
		return nil, nil // Error checking, let other publishers try
	}
	if !hasVolume {
		return nil, nil // Not our volume
	}

	// Unmount the library from the target path
	targetPath := req.GetTargetPath()
	err = bindUnmount(s.fs, s.mounter, targetPath)
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: ""},
			fmt.Errorf("failed to unmount library: %w", err)
	}

	// Remove volume tracking (this will also delete the library from disk if no longer used)
	err = s.libraryManager.RemoveVolume(context.Background(), volumeID)
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: ""},
			fmt.Errorf("failed to remove volume tracking: %w", err)
	}

	return &PublisherResponse{VolumeType: DatadogLibrary, VolumePath: ""}, nil
}

// getLibraryPath downloads the library if needed and returns the local path to mount.
// The returned path includes the source subdirectory from the volume context.
// Returns the path and the image reference for metrics.
func (s libraryPublisher) getLibraryPath(volumeCtx map[string]string, volumeID string) (path, image string, err error) {
	pkg := volumeCtx[keyLibraryPackage]
	registry := volumeCtx[keyLibraryRegistry]
	version := volumeCtx[keyLibraryVersion]

	lib, err := librarymanager.NewLibrary(pkg, registry, version, true)
	if err != nil {
		return "", "", fmt.Errorf("invalid library configuration: %w", err)
	}

	basePath, err := s.libraryManager.GetLibraryForVolume(context.Background(), volumeID, lib)
	if err != nil {
		return "", lib.Image(), fmt.Errorf("failed to get library for volume: %w", err)
	}

	// Append the source path to mount only the requested subdirectory
	source := languageLibrarySourcePath
	if pkg == "apm-inject" {
		source = injectorLibrarySourcePath
	}

	// Remove leading slash from source to join paths correctly
	source = strings.TrimPrefix(source, "/")
	path = filepath.Join(basePath, source)

	return path, lib.Image(), nil
}

func newLibraryPublisher(fs afero.Afero, mounter mount.Interface, libraryManager *librarymanager.LibraryManager) Publisher {
	return libraryPublisher{fs: fs, mounter: mounter, libraryManager: libraryManager}
}
