// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

const (
	// Default content for ld.so.preload
	defaultPreloadContent = "/opt/datadog-packages/datadog-apm-inject/stable/inject/launcher.preload.so\n"
)

// injectorPreloadPublisher mounts an ld.so.preload file into containers.
type injectorPreloadPublisher struct {
	fs      afero.Afero
	mounter mount.Interface
	// preloadFilePath is the path to the preload file
	preloadFilePath string
	// mu protects the check-and-create operation
	mu sync.Mutex
}

func (p *injectorPreloadPublisher) ensurePreloadFileExists() error {
	// Fast path: check without lock
	if exists, err := p.fs.Exists(p.preloadFilePath); err != nil || exists {
		return err
	}

	// Slow path: lock and double-check before creating
	p.mu.Lock()
	defer p.mu.Unlock()

	if exists, err := p.fs.Exists(p.preloadFilePath); err != nil || exists {
		return err
	}

	return p.fs.WriteFile(p.preloadFilePath, []byte(defaultPreloadContent), 0644)
}

func (p *injectorPreloadPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()
	if VolumeType(volumeCtx["type"]) != DatadogInjectorPreload {
		return nil, nil // Not our volume
	}

	// Defensive code: injector preload volumes must be mounted in read-only mode to protect the shared store
	if !req.GetReadonly() {
		return &PublisherResponse{VolumeType: DatadogInjectorPreload}, fmt.Errorf("injector preload volumes must be mounted in read-only mode")
	}

	targetPath := req.GetTargetPath()

	// Ensure the preload file exists
	if err := p.ensurePreloadFileExists(); err != nil {
		return &PublisherResponse{VolumeType: DatadogInjectorPreload}, err
	}

	// Bind mount the file to the target path
	if err := bindMount(p.fs, p.mounter, p.preloadFilePath, targetPath, true); err != nil {
		return &PublisherResponse{VolumeType: DatadogInjectorPreload},
			fmt.Errorf("failed to mount preload file: %w", err)
	}

	return &PublisherResponse{VolumeType: DatadogInjectorPreload, VolumePath: targetPath}, nil
}

func (p *injectorPreloadPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil // Handled by unmountPublisher
}

func newInjectorPreloadPublisher(fs afero.Afero, mounter mount.Interface, storageBasePath string) Publisher {
	return &injectorPreloadPublisher{
		fs:              fs,
		mounter:         mounter,
		preloadFilePath: filepath.Join(storageBasePath, "ld.so.preload"),
	}
}
