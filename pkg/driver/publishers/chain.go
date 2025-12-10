// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog"
)

type chainPublisher struct {
	publishers []Publisher
}

func (s chainPublisher) Stage(req *csi.NodeStageVolumeRequest) (bool, error) {
	for _, publisher := range s.publishers {
		supported, err := publisher.Stage(req)
		if err != nil {
			klog.Infof("failed to stage volume with publisher %T: %v", publisher, err)
		}
		if supported {
			return supported, err
		}
	}
	return false, nil
}

func (s chainPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (bool, error) {
	for _, publisher := range s.publishers {
		supported, err := publisher.Unstage(req)
		if err != nil {
			klog.Infof("failed to unstage volume with publisher %T: %v", publisher, err)
		}
		if supported {
			return supported, err
		}
	}
	return false, nil
}

func (s chainPublisher) Publish(req *csi.NodePublishVolumeRequest) (bool, error) {
	for _, publisher := range s.publishers {
		supported, err := publisher.Publish(req)
		if err != nil {
			klog.Infof("failed to publish volume with publisher %T: %v", publisher, err)
		}
		if supported {
			return supported, err
		}
	}
	return false, nil
}

func (s chainPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (bool, error) {
	for _, publisher := range s.publishers {
		supported, err := publisher.Unpublish(req)
		if err != nil {
			klog.Infof("failed to unpublish volume with publisher %T: %v", publisher, err)
		}
		if supported {
			return supported, err
		}
	}
	return false, nil
}

func newChainPublisher(publishers ...Publisher) Publisher {
	return chainPublisher{publishers: publishers}
}
