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
	name       string
	version    string
	publishers map[publishers.PublisherKind]publishers.Publisher
	fs         afero.Afero
	mounter    mount.Interface
}

// NewDatadogCSIDriver builds and returns a new Datadog CSI driver
func NewDatadogCSIDriver(name string) (*DatadogCSIDriver, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.New("")

	return &DatadogCSIDriver{
		name: name,

		// hardcoded for now
		// TODO: allow setting version dynamically on build
		version: "v1",

		publishers: publishers.GetPublishers(fs, mounter),
		fs:         fs,
		mounter:    mounter,
	}, nil
}
