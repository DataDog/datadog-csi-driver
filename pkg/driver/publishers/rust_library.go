// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"
	"os"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

// rustLibraryPublisher handles DatadogRustLibraries volumes. It reuses the
// DatadogLibrary download + bind-mount flow but tightens the extracted files to
// 0500 (owner-only). The Agent shared-library check loader rejects a check .so
// that grants any group/other access; the shared archive extractor writes 0755
// (correct for APM tracer libraries, which any app uid must read), so Rust check
// bundles need their own volume type with restrictive permissions.
type rustLibraryPublisher struct {
	libraryPublisher
}

func newRustLibraryPublisher(fs afero.Afero, mounter mount.Interface, libraryManager *librarymanager.LibraryManager, disabled bool, allowedRegistries []string) Publisher {
	return rustLibraryPublisher{libraryPublisher{
		fs:                fs,
		mounter:           mounter,
		libraryManager:    libraryManager,
		disabled:          disabled,
		allowedRegistries: allowedRegistries,
	}}
}

// Publish downloads the library, tightens the extracted files to 0500, and
// bind-mounts the package directory read-only.
func (s rustLibraryPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()
	if VolumeType(volumeCtx["type"]) != DatadogRustLibraries {
		return nil, nil // Not our volume
	}

	if s.disabled {
		return &PublisherResponse{VolumeType: DatadogRustLibraries}, fmt.Errorf("SSI is disabled, rust library volumes cannot be published")
	}

	// Read-only protects the shared store from tampering between check and dlopen.
	if !req.GetReadonly() {
		return &PublisherResponse{VolumeType: DatadogRustLibraries}, fmt.Errorf("rust library volumes must be mounted in read-only mode")
	}

	registry := volumeCtx[keyLibraryRegistry]
	if !s.registryAllowed(registry) {
		return &PublisherResponse{VolumeType: DatadogRustLibraries}, fmt.Errorf("registry %q is not in the allow list", registry)
	}

	libraryPath, image, err := s.getLibraryPath(volumeCtx, req.GetVolumeId())
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogRustLibraries, VolumePath: image}, err
	}

	// Tighten to owner-only so the Agent shared-library loader accepts the .so.
	// Safe to do in place: the store is keyed by image digest and these bundles
	// carry only check libraries, never APM tracer libraries.
	if err := chmodRegularFiles(s.fs, libraryPath, 0o500); err != nil {
		return &PublisherResponse{VolumeType: DatadogRustLibraries, VolumePath: image}, fmt.Errorf("failed to set restrictive permissions: %w", err)
	}

	err = bindMount(s.fs, s.mounter, bindMountArgs{
		hostPath:   libraryPath,
		targetPath: req.GetTargetPath(),
		isFile:     false,
		readOnly:   true,
	})
	if err != nil {
		return &PublisherResponse{VolumeType: DatadogRustLibraries, VolumePath: image}, err
	}

	return &PublisherResponse{VolumeType: DatadogRustLibraries, VolumePath: image}, nil
}

// chmodRegularFiles sets mode on every regular file under root, leaving
// directories and symlinks untouched.
func chmodRegularFiles(fs afero.Afero, root string, mode os.FileMode) error {
	return afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return fs.Chmod(path, mode)
		}
		return nil
	})
}
