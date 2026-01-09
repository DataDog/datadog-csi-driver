// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

// mockPublisher is a test helper that implements Publisher interface
type mockPublisher struct {
	publishResp   *PublisherResponse
	publishErr    error
	unpublishResp *PublisherResponse
	unpublishErr  error
}

func (m mockPublisher) Publish(req *csi.NodePublishVolumeRequest) (*PublisherResponse, error) {
	return m.publishResp, m.publishErr
}

func (m mockPublisher) Unpublish(req *csi.NodeUnpublishVolumeRequest) (*PublisherResponse, error) {
	return m.unpublishResp, m.unpublishErr
}

func TestChainPublisher_Publish_StopsAtFirstResponse(t *testing.T) {
	firstResp := &PublisherResponse{VolumeType: "First", VolumePath: "/first"}
	secondResp := &PublisherResponse{VolumeType: "Second", VolumePath: "/second"}

	chain := newChainPublisher(
		mockPublisher{publishResp: nil},        // skipped
		mockPublisher{publishResp: firstResp},  // stops here
		mockPublisher{publishResp: secondResp}, // never reached
	)

	resp, err := chain.Publish(&csi.NodePublishVolumeRequest{})

	assert.NoError(t, err)
	assert.Equal(t, firstResp, resp)
}

func TestChainPublisher_Publish_ReturnsNilIfNoPublisherMatches(t *testing.T) {
	chain := newChainPublisher(
		mockPublisher{publishResp: nil},
		mockPublisher{publishResp: nil},
	)

	resp, err := chain.Publish(&csi.NodePublishVolumeRequest{})

	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func TestChainPublisher_Publish_ReturnsErrorWithResponse(t *testing.T) {
	expectedErr := errors.New("publish failed")
	expectedResp := &PublisherResponse{VolumeType: "Failed", VolumePath: "/failed"}

	chain := newChainPublisher(
		mockPublisher{publishResp: nil},
		mockPublisher{publishResp: expectedResp, publishErr: expectedErr},
	)

	resp, err := chain.Publish(&csi.NodePublishVolumeRequest{})

	assert.Equal(t, expectedErr, err)
	assert.Equal(t, expectedResp, resp)
}

func TestChainPublisher_Unpublish_StopsAtFirstResponse(t *testing.T) {
	firstResp := &PublisherResponse{VolumeType: "First", VolumePath: "/first"}

	chain := newChainPublisher(
		mockPublisher{unpublishResp: nil},
		mockPublisher{unpublishResp: firstResp},
		mockPublisher{unpublishResp: &PublisherResponse{VolumeType: "Never", VolumePath: "/never"}},
	)

	resp, err := chain.Unpublish(&csi.NodeUnpublishVolumeRequest{})

	assert.NoError(t, err)
	assert.Equal(t, firstResp, resp)
}

func TestChainPublisher_EmptyChain(t *testing.T) {
	chain := newChainPublisher()

	t.Run("Publish returns nil", func(t *testing.T) {
		resp, err := chain.Publish(&csi.NodePublishVolumeRequest{})
		assert.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("Unpublish returns nil", func(t *testing.T) {
		resp, err := chain.Unpublish(&csi.NodeUnpublishVolumeRequest{})
		assert.NoError(t, err)
		assert.Nil(t, resp)
	})
}
