// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/utils/mount"
)

// unmountPublisher is a fallback publisher that handles all Unpublish requests.
// Since NodeUnpublishVolumeRequest doesn't include VolumeContext, we cannot
// determine which publisher originally handled the Publish. The unmount logic
// is identical for all bind mounts, so this publisher acts as the final handler.
type unmountPublisher struct {
	mounter mount.Interface
}

func (s unmountPublisher) Stage(req *csi.NodeStageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s unmountPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s unmountPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	return nil, nil
}

func (s unmountPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	// Unpublish doesn't have VolumeContext, so we return an empty response
	return &PublisherResponse{}, bindUnmount(s.mounter, req.GetTargetPath())
}

func newUnmountPublisher(mounter mount.Interface) Publisher {
	return unmountPublisher{mounter: mounter}
}
