// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager_test

import (
	"testing"

	"github.com/Datadog/datadog-csi-driver/pkg/librarymanager"
	"github.com/stretchr/testify/require"
)

func TestNewLibrary(t *testing.T) {
	tests := map[string]struct {
		name          string
		registry      string
		version       string
		pull          bool
		wantErr       bool
		expectedImage string
	}{
		"good input produces no error": {
			name:          "foo",
			registry:      "bar",
			version:       "zed",
			pull:          true,
			expectedImage: "bar/foo:zed",
		},
		"tag version uses colon separator": {
			name:          "dd-lib-python-init",
			registry:      "gcr.io/datadoghq",
			version:       "v1.2.3",
			pull:          true,
			expectedImage: "gcr.io/datadoghq/dd-lib-python-init:v1.2.3",
		},
		"sha256 digest version uses @ separator": {
			name:          "dd-lib-python-init",
			registry:      "gcr.io/datadoghq",
			version:       "sha256:abc123def456",
			pull:          true,
			expectedImage: "gcr.io/datadoghq/dd-lib-python-init@sha256:abc123def456",
		},
		"sha384 digest version uses @ separator": {
			name:          "dd-lib-python-init",
			registry:      "gcr.io/datadoghq",
			version:       "sha384:abc123def456",
			pull:          true,
			expectedImage: "gcr.io/datadoghq/dd-lib-python-init@sha384:abc123def456",
		},
		"sha512 digest version uses @ separator": {
			name:          "dd-lib-python-init",
			registry:      "gcr.io/datadoghq",
			version:       "sha512:abc123def456",
			pull:          true,
			expectedImage: "gcr.io/datadoghq/dd-lib-python-init@sha512:abc123def456",
		},
		"tag@digest combo uses : separator": {
			name:          "dd-lib-python-init",
			registry:      "gcr.io/datadoghq",
			version:       "v1.2.3@sha256:abc123def456",
			pull:          true,
			expectedImage: "gcr.io/datadoghq/dd-lib-python-init:v1.2.3@sha256:abc123def456",
		},
		"empty name causes error": {
			name:     "",
			registry: "bar",
			version:  "zed",
			pull:     false,
			wantErr:  true,
		},
		"empty registry causes error": {
			name:     "foo",
			registry: "",
			version:  "zed",
			pull:     false,
			wantErr:  true,
		},
		"empty version causes error": {
			name:     "foo",
			registry: "bar",
			version:  "",
			pull:     false,
			wantErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			lib, err := librarymanager.NewLibrary(test.name, test.registry, test.version, test.pull)
			if test.wantErr {
				require.Error(t, err, "error was expected")
				return
			}
			require.NoError(t, err, "no error was expected")
			require.Equal(t, test.expectedImage, lib.Image())
			require.Equal(t, test.pull, lib.Pull())
		})
	}
}
