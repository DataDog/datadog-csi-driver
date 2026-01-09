// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"fmt"

	"github.com/Datadog/datadog-csi-driver/pkg/driver/publishers"
	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
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
	return driver.libraryManager.Stop()
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name, apmHostSocketPath, dsdHostSocketPath, storageBasePath, version string) (*DatadogCSIDriver, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.New("")

	// Ensure the storage base path exists
	if err := fs.MkdirAll(storageBasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage base path: %w", err)
	}

	lm, err := librarymanager.NewLibraryManager(storageBasePath)
	if err != nil {
		return nil, err
	}

	return &DatadogCSIDriver{
		name:    name,
		version: version,

		publisher:      publishers.GetPublishers(fs, mounter, apmHostSocketPath, dsdHostSocketPath, storageBasePath, lm),
		libraryManager: lm,
		fs:             fs,
		mounter:        mounter,
	}, nil
}
