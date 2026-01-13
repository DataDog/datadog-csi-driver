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
	// initOnce ensures the preload file is created only once
	initOnce sync.Once
	initErr  error
}

func (p *injectorPreloadPublisher) ensurePreloadFile() error {
	p.initOnce.Do(func() {
		// Create the preload file if it doesn't exist
		exists, err := p.fs.Exists(p.preloadFilePath)
		if err != nil {
			p.initErr = fmt.Errorf("failed to check preload file: %w", err)
			return
		}
		if !exists {
			if err := p.fs.WriteFile(p.preloadFilePath, []byte(defaultPreloadContent), 0644); err != nil {
				p.initErr = fmt.Errorf("failed to write preload file: %w", err)
				return
			}
		}
	})
	return p.initErr
}

func (p *injectorPreloadPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	volumeCtx := req.GetVolumeContext()
	if VolumeType(volumeCtx["type"]) != DatadogInjectorPreload {
		return nil, nil // Not our volume
	}

	targetPath := req.GetTargetPath()

	// Ensure the preload file exists
	if err := p.ensurePreloadFile(); err != nil {
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
