// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"fmt"
	log "log/slog"
	"strings"
	"time"

	"github.com/Datadog/datadog-csi-driver/pkg/driver/publishers"
	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

const (
	// cleanupDelay is the delay before cleaning up unused libraries.
	cleanupDelay = 15 * time.Minute
)

// DatadogCSIDriver is datadog CSI driver implementing CSI Node and Identity Server
type DatadogCSIDriver struct {
	csi.UnimplementedNodeServer
	csi.UnimplementedIdentityServer
	name    string
	version string

	publisher      publishers.Publisher
	libraryManager *librarymanager.LibraryManager
	fs             afero.Afero
	mounter        mount.Interface
}

// Version returns the CSI driver version
func (driver *DatadogCSIDriver) Version() string {
	return driver.version
}

// Stop ensures all dependencies are stopped correctly.
func (driver *DatadogCSIDriver) Stop() error {
	if driver.libraryManager == nil {
		return nil
	}
	return driver.libraryManager.Stop()
}

func createStorageDir(fs afero.Afero, storageBasePath string) (string, error) {
	storageBasePath = strings.TrimSpace(storageBasePath)
	if storageBasePath == "" {
		return "", nil
	}

	if err := fs.MkdirAll(storageBasePath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create storage base path: %w", err)
	}

	probeDir, err := afero.TempDir(fs, storageBasePath, ".write-check-*")
	if err != nil {
		return "", fmt.Errorf("storage base path is not writable: %w", err)
	}
	_ = fs.RemoveAll(probeDir)

	return storageBasePath, nil
}

func newDatadogCSIDriver(
	fs afero.Afero,
	mounter mount.Interface,
	logger *log.Logger,
	name, apmHostSocketPath, dsdHostSocketPath, storageBasePath, version string,
	apmEnabled bool,
	allowedRegistries []string,
) (*DatadogCSIDriver, error) {
	requestedStorageBasePath := storageBasePath

	var err error
	storageBasePath, err = createStorageDir(fs, storageBasePath)
	if err != nil {
		logger.Warn("Disabling SSI storage", "storage_base_path", requestedStorageBasePath, "error", err)
		storageBasePath = ""
	}

	var lm *librarymanager.LibraryManager
	if storageBasePath != "" {
		lm, err = librarymanager.NewLibraryManager(
			storageBasePath,
			librarymanager.WithFilesystem(fs),
			librarymanager.WithCleanupStrategy(librarymanager.NewDelayedCleanupStrategy(cleanupDelay)),
		)
		if err != nil {
			return nil, err
		}
	}

	return &DatadogCSIDriver{
		name:    name,
		version: version,

		publisher:      publishers.GetPublishers(fs, mounter, apmHostSocketPath, dsdHostSocketPath, storageBasePath, lm, apmEnabled, allowedRegistries),
		libraryManager: lm,
		fs:             fs,
		mounter:        mounter,
	}, nil
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name, apmHostSocketPath, dsdHostSocketPath, storageBasePath, version string, apmEnabled bool, allowedRegistries []string) (*DatadogCSIDriver, error) {
	return newDatadogCSIDriver(
		afero.Afero{Fs: afero.NewOsFs()},
		mount.New(""),
		log.Default(),
		name,
		apmHostSocketPath,
		dsdHostSocketPath,
		storageBasePath,
		version,
		apmEnabled,
		allowedRegistries,
	)
}
