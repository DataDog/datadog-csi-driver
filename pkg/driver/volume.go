// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog/v2"
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

	// DatadogSocketsDirectory mounts the parent directory of the dogstatsd socket
	// This option is deprecated, but kept to avoid breaking backward compatibility
	DatadogSocketsDirectory VolumeType = "DatadogSocketsDirectory"

	APMLibraryDirectory VolumeType = "APMLibraryDirectory"
)

type Mode string

const (
	// ModeSocket is the socket mode, equivalent to Socket hostpath volumes
	ModeSocket Mode = "socket"
	// ModeLocal is the local mode, equivalent to Directory hostpath volumes
	ModeLocal = "local"
	ModeOCI   = "oci"
)

func getModeAndPath(volumeType VolumeType, apmHostSocketPath, dsdHostSocketPath, apmLibraryPath string) (mode Mode, path string, err error) {
	switch volumeType {
	case APMSocket:
		path = apmHostSocketPath
		mode = ModeSocket
	case APMLibraryDirectory:
		path = filepath.Dir(apmLibraryPath)
		mode = ModeOCI
	case APMSocketDirectory:
		path = filepath.Dir(apmHostSocketPath)
		mode = ModeLocal
	case DSDSocket:
		path = dsdHostSocketPath
		mode = ModeSocket
	case DSDSocketDirectory:
		path = filepath.Dir(dsdHostSocketPath)
		mode = ModeLocal
	case DatadogSocketsDirectory:
		klog.Warningf("%s volume type is deprecated. Prefer using %s or %s instead.", DatadogSocketsDirectory, DSDSocketDirectory, APMSocketDirectory)
		path = filepath.Dir(dsdHostSocketPath)
		mode = ModeLocal
	default:
		err = fmt.Errorf("unsupported volume type %q", volumeType)
	}

	return
}

// DDVolumeRequest encapsulates the properties of a request for datadog CSI volume
type DDVolumeRequest struct {
	volumeId      string
	targetpath    string
	volumeType    VolumeType
	mode          Mode
	path          string
	volumeContext map[string]string
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

	// Kept to avoid breaking backwards compatibility
	volumePath, foundPath := volumeCtx["path"]
	volumeMode, foundMode := volumeCtx["mode"]

	var mode Mode
	var path string

	if foundType {
		// Using new schema
		var err error
		mode, path, err = getModeAndPath(VolumeType(volumeType), d.apmHostSocketPath, d.dsdHostSocketPath, d.apmLibraryPath)
		if err != nil {
			return nil, err
		}
	} else if foundMode && foundPath {
		isValidSocket := volumeMode == string(ModeSocket) && (volumePath == d.apmHostSocketPath || volumePath == d.dsdHostSocketPath)
		isValidDir := volumeMode == string(ModeLocal) && (volumePath == filepath.Dir(d.apmHostSocketPath) || volumePath == filepath.Dir(d.dsdHostSocketPath) || volumePath == filepath.Dir(d.apmLibraryPath))

		if isValidDir || isValidSocket {
			mode = Mode(volumeMode)
			path = volumePath
		} else {
			return nil, fmt.Errorf("unexpected volume attributes: permitted values are [mode:%s path:%s], [mode:%s path:%s], [mode:%s path:%s], [mode:%s path:%s], [mode:%s path:%s]",
				ModeLocal,
				filepath.Dir(d.apmHostSocketPath),
				ModeLocal,
				filepath.Dir(d.dsdHostSocketPath),
				ModeSocket,
				d.apmHostSocketPath,
				ModeSocket,
				d.dsdHostSocketPath,
				ModeOCI,
				d.apmLibraryPath,
			)
		}
	} else {
		return nil, errors.New("missing property 'type' in CSI volume context")
	}

	return &DDVolumeRequest{
		volumeId:      volumeId,
		targetpath:    targetPath,
		volumeType:    VolumeType(volumeType),
		mode:          mode,
		path:          path,
		volumeContext: req.VolumeContext,
	}, nil
}
