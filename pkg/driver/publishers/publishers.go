// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

// Publisher defines logic for mounting and unmounting volumes.
type Publisher interface {
	// Mount mounts the hostPath at targetPath
	Mount(targetPath string, hostPath string, volumeContext map[string]string) error
}

// PublisherKind represents the type of the publisher
type PublisherKind string

const (
	// Socket is the publisher kind that allows mounting UDS sockets.
	Socket PublisherKind = "socket"
	// Local is the publichser kind that allows mounting local directories.
	Local = "local"
	OCI   = "oci"
)

func GetPublishers(fs afero.Afero, mounter mount.Interface) map[PublisherKind]Publisher {
	return map[PublisherKind]Publisher{
		Socket: newSocketPublisher(fs, mounter),
		Local:  newLocalPublisher(fs, mounter),
		OCI:    newOCIPublisher(fs, mounter),
	}
}
