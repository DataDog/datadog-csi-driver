// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog"
)

// chainPublisher is a publisher that chains multiple publishers together.
// It stops at the first publisher that returns a non-nil response.
type chainPublisher struct {
	publishers []Publisher
}

func (s chainPublisher) Stage(req *csi.NodeStageVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Stage(req)
		if err != nil {
			klog.Infof("failed to stage volume with publisher %T: %v", publisher, err)
		}
		if resp != nil {
			return resp, err
		}
	}
	return nil, nil
}

func (s chainPublisher) Unstage(req *csi.NodeUnstageVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Unstage(req)
		if err != nil {
			klog.Infof("failed to unstage volume with publisher %T: %v", publisher, err)
		}
		if resp != nil {
			return resp, err
		}
	}
	return nil, nil
}

func (s chainPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Publish(req)
		if err != nil {
			klog.Infof("failed to publish volume with publisher %T: %v", publisher, err)
		}
		if resp != nil {
			return resp, err
		}
	}
	return nil, nil
}

func (s chainPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Unpublish(req)
		if err != nil {
			klog.Infof("failed to unpublish volume with publisher %T: %v", publisher, err)
		}
		if resp != nil {
			return resp, err
		}
	}
	return nil, nil
}

func newChainPublisher(publishers ...Publisher) Publisher {
	return chainPublisher{publishers: publishers}
}
