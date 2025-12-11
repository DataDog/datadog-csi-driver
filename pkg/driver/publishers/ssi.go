// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"k8s.io/utils/mount"
)

type ssiPublisher struct {
	fs      afero.Afero
	mounter mount.Interface
}

func (s ssiPublisher) Stage(req *csi.NodeStageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s ssiPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s ssiPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s ssiPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func newSSIPublisher(fs afero.Afero, mounter mount.Interface) Publisher {
	return ssiPublisher{fs: fs, mounter: mounter}
}
