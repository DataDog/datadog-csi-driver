// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/rs/zerolog/log"
)

// chainPublisher is a publisher that chains multiple publishers together.
// It stops at the first publisher that returns a non-nil response.
type chainPublisher struct {
	publishers []Publisher
}

func (s chainPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Publish(req)
		if err != nil {
			log.Info().Err(err).Str("publisher", typeToString(publisher)).Msg("failed to publish volume with publisher")
		}
		if resp != nil {
			return resp, err
		}
	}
	return nil, nil
}

func typeToString(v interface{}) string {
	return fmt.Sprintf("%T", v)
}

func (s chainPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	for _, publisher := range s.publishers {
		resp, err := publisher.Unpublish(req)
		if err != nil {
			log.Info().Err(err).Str("publisher", typeToString(publisher)).Msg("failed to unpublish volume with publisher")
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
