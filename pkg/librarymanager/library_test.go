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
