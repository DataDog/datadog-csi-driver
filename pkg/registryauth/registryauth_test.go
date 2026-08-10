// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package registryauth

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"
)

func TestKeychainMergesEnvironmentCredentials(t *testing.T) {
	t.Setenv(envPrefix+"0", `{"auths":{"first.example.com":{"username":"alice","password":"first"}}}`)
	t.Setenv(envPrefix+"1", `{"auths":{"https://second.example.com/v1/":{"username":"bob","password":"second"}}}`)

	keychain, err := NewKeychainFromEnvironment()
	require.NoError(t, err)
	for repository, expectedUser := range map[string]string{
		"first.example.com/datadog/library":  "alice",
		"second.example.com/datadog/library": "bob",
	} {
		target, err := name.NewRepository(repository)
		require.NoError(t, err)
		authenticator, err := keychain.Resolve(target)
		require.NoError(t, err)
		config, err := authenticator.Authorization()
		require.NoError(t, err)
		require.Equal(t, expectedUser, config.Username)
	}
}

func TestKeychainRejectsInvalidEnvironmentCredentials(t *testing.T) {
	t.Setenv(envPrefix+"0", `invalid`)

	_, err := NewKeychainFromEnvironment()
	require.Error(t, err)
}
