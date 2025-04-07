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

	// datadogSocketsHostPath is the directory in the which the datadog sockets can be found on the host
	datadogSocketsHostPath string

	// Dogstatsd socket file name
	dsdHostSocketFileName string

	// APM socket file name
	apmHostSocketFileName string

	publishers map[publishers.PublisherKind]publishers.Publisher
	fs         afero.Afero
	mounter    mount.Interface
}

// Version returns the CSI driver version
func (driver *DatadogCSIDriver) Version() string {
	return driver.version
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name, datadogSocketsHostPath, dsdHostSocketFileName, apmHostSocketFileName, version string) (*DatadogCSIDriver, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.New("")

	return &DatadogCSIDriver{
		name: name,

		version: version,

		datadogSocketsHostPath: datadogSocketsHostPath,
		dsdHostSocketFileName:  dsdHostSocketFileName,
		apmHostSocketFileName:  apmHostSocketFileName,

		publishers: publishers.GetPublishers(fs, mounter),
		fs:         fs,
		mounter:    mounter,
	}, nil
}
