package driver

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type VolumeType string

const (
	// APMSocket mounts the apm socket file after verifying the existence of the file
	APMSocket VolumeType = "APMSocket"
	// DSDSocket mounts the dogstatsd socket file after verifying the existence of the file
	DSDSocket = "DSDSocket"
	// DatadogSocketsDirectory mounts the directory containing the apm and dogstatsd sockets
	DatadogSocketsDirectory = "DatadogSocketsDirectory"
)

type Mode string

const (
	// ModeSocket is the socket mode, equivalent to Socket hostpath volumes
	ModeSocket Mode = "socket"
	// ModeLocal is the local mode, equivalent to Directory hostpath volumes
	ModeLocal = "local"
)

func getModeAndPath(volumeType VolumeType, datadogSocketsDirectory, dsdSocketFileName, apmSocketFileName string) (mode Mode, path string, err error) {
	switch volumeType {
	case APMSocket:
		path = filepath.Join(datadogSocketsDirectory, apmSocketFileName)
		mode = ModeSocket
	case DSDSocket:
		path = filepath.Join(datadogSocketsDirectory, dsdSocketFileName)
		mode = ModeSocket
	case DatadogSocketsDirectory:
		path = datadogSocketsDirectory
		mode = ModeLocal
	default:
		err = fmt.Errorf("unsupported volume type %q", volumeType)
	}

	return
}

// DDVolumeRequest encapsulates the properties of a request for datadog CSI volume
type DDVolumeRequest struct {
	volumeId   string
	targetpath string
	volumeType VolumeType
	mode       Mode
	path       string
}

// NewDDVolumeRequest builds a DDVolumeRequest object based on csi node publish volume request
func (d *DatadogCSIDriver) DDVolumeRequest(req *csi.NodePublishVolumeRequest) (*DDVolumeRequest, error) {
	if req == nil {
		return nil, errors.New("nil node publish volume request")
	}

	targetPath := req.GetTargetPath()
	volumeId := req.GetVolumeId()
	volumeCtx := req.GetVolumeContext()
	volumeType, foundType := volumeCtx["type"]
	if !foundType {
		return nil, errors.New("missing property 'type' in CSI volume context")
	}

	mode, path, err := getModeAndPath(VolumeType(volumeType), d.datadogSocketsHostPath, d.dsdHostSocketFileName, d.apmHostSocketFileName)
	if err != nil {
		return nil, err
	}

	return &DDVolumeRequest{
		volumeId:   volumeId,
		targetpath: targetPath,
		volumeType: VolumeType(volumeType),
		mode:       mode,
		path:       path,
	}, nil
}
