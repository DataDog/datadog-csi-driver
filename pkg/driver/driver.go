// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"github.com/Datadog/datadog-csi-driver/pkg/driver/publishers"

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

	apmHostSocketPath string
	dsdHostSocketPath string
	apmLibraryPath    string

	publishers map[publishers.PublisherKind]publishers.Publisher
	fs         afero.Afero
	mounter    mount.Interface
}

// Version returns the CSI driver version
func (driver *DatadogCSIDriver) Version() string {
	return driver.version
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name, apmHostSocketPath, dsdHostSocketPath, apmLibraryPath, version string) (*DatadogCSIDriver, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.New("")

	return &DatadogCSIDriver{
		name: name,

		version: version,

		apmHostSocketPath: apmHostSocketPath,
		dsdHostSocketPath: dsdHostSocketPath,
		apmLibraryPath:    apmLibraryPath,

		publishers: publishers.GetPublishers(fs, mounter),
		fs:         fs,
		mounter:    mounter,
	}, nil
}
