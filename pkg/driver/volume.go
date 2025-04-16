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

	// APMSocketDirectory mounts the parent directory of the apm socket
	APMSocketDirectory VolumeType = "APMSocketDirectory"

	// DSDSocket mounts the dogstatsd socket file after verifying the existence of the file
	DSDSocket VolumeType = "DSDSocket"

	// DSDSocketDirectory mounts the parent directory of the dogstatsd socket
	DSDSocketDirectory VolumeType = "DSDSocketDirectory"
)

type Mode string

const (
	// ModeSocket is the socket mode, equivalent to Socket hostpath volumes
	ModeSocket Mode = "socket"
	// ModeLocal is the local mode, equivalent to Directory hostpath volumes
	ModeLocal = "local"
)

func getModeAndPath(volumeType VolumeType, apmHostSocketPath, dsdHostSocketPath string) (mode Mode, path string, err error) {
	switch volumeType {
	case APMSocket:
		path = apmHostSocketPath
		mode = ModeSocket
	case APMSocketDirectory:
		path = filepath.Dir(apmHostSocketPath)
		mode = ModeLocal
	case DSDSocket:
		path = dsdHostSocketPath
		mode = ModeSocket
	case DSDSocketDirectory:
		path = filepath.Dir(dsdHostSocketPath)
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

	mode, path, err := getModeAndPath(VolumeType(volumeType), d.apmHostSocketPath, d.dsdHostSocketPath)
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
