// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/mount"
)

func TestGetPublishers_SkipsSSIPublishersWhenStorageBasePathIsEmpty(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	mounter := mount.NewFakeMounter(nil)

	publisher := GetPublishers(fs, mounter, "/tmp/apm.sock", "/tmp/dsd.sock", "", nil, true)

	t.Run("library volume is ignored", func(t *testing.T) {
		resp, err := publisher.Publish(&csi.NodePublishVolumeRequest{
			VolumeId:   "library-volume",
			TargetPath: "/target/library",
			Readonly:   true,
			VolumeContext: map[string]string{
				"type": string(DatadogLibrary),
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("injector preload volume is ignored", func(t *testing.T) {
		resp, err := publisher.Publish(&csi.NodePublishVolumeRequest{
			VolumeId:   "preload-volume",
			TargetPath: "/target/ld.so.preload",
			Readonly:   true,
			VolumeContext: map[string]string{
				"type": string(DatadogInjectorPreload),
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, resp)
	})
}
