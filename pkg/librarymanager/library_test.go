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

func TestLibrary(t *testing.T) {
	tests := map[string]struct {
		name           string
		registry       string
		version        string
		path           string
		pull           bool
		wantErr        bool
		expectedSource string
		expectedImage  string
	}{
		"good input produces no error": {
			name:           "foo",
			registry:       "bar",
			version:        "zed",
			path:           "idk",
			pull:           true,
			expectedSource: "idk",
			expectedImage:  "bar/foo:zed",
		},
		"empty name causes error": {
			name:     "",
			registry: "bar",
			version:  "zed",
			path:     "idk",
			pull:     false,
			wantErr:  true,
		},
		"empty registry causes error": {
			name:     "foo",
			registry: "",
			version:  "zed",
			path:     "idk",
			pull:     false,
			wantErr:  true,
		},
		"empty version causes error": {
			name:     "foo",
			registry: "bar",
			version:  "",
			path:     "idk",
			pull:     false,
			wantErr:  true,
		},
		"empty path causes error": {
			name:     "foo",
			registry: "bar",
			version:  "zed",
			path:     "",
			pull:     false,
			wantErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			lib, err := librarymanager.NewLibrary(test.name, test.registry, test.version, test.path, test.pull)
			if test.wantErr {
				require.Error(t, err, "error was expected")
				return
			}
			require.NoError(t, err, "no error was expected")
			require.Equal(t, test.expectedSource, lib.Source())
			require.Equal(t, test.expectedImage, lib.Image())
			require.Equal(t, test.pull, lib.Pull())
		})
	}
}
