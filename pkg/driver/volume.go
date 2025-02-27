package driver

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

// DDVolumeRequest encapsulates the properties of a request for datadog CSI volume
type DDVolumeRequest struct {
	volumeId   string
	targetpath string
	mode       string
	path       string
}

// NewDDVolumeRequest builds a DDVolumeRequest object based on csi node publish volume request
func NewDDVolumeRequest(req *csi.NodePublishVolumeRequest) (*DDVolumeRequest, error) {
	if req == nil {
		return nil, errors.New("nil node publish volume request")
	}

	targetPath := req.GetTargetPath()
	volumeId := req.GetVolumeId()
	volumeCtx := req.GetVolumeContext()
	mode, foundMode := volumeCtx["mode"]
	hostpath, foundHostPath := volumeCtx["path"]

	if !foundMode {
		return nil, errors.New("missing property 'mode' in CSI volume context")
	}

	if !foundHostPath {
		return nil, errors.New("missing property 'path' in CSI volume context")
	}

	return &DDVolumeRequest{
		volumeId:   volumeId,
		targetpath: targetPath,
		mode:       mode,
		path:       hostpath,
	}, nil
}
