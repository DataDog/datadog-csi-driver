// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package registryauth

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

const envPrefix = "DD_APM_REGISTRY_AUTH_"

// NewKeychainFromEnvironment creates a keychain from all indexed registry
// authentication environment variables.
func NewKeychainFromEnvironment() (authn.Keychain, error) {
	config := configfile.New("")
	hasCredentials := false
	for _, env := range os.Environ() {
		name, value, ok := strings.Cut(env, "=")
		if ok && strings.HasPrefix(name, envPrefix) {
			if err := config.LoadFromReader(strings.NewReader(value)); err != nil {
				return nil, fmt.Errorf("invalid registry credentials: %w", err)
			}
			hasCredentials = true
		}
	}
	if !hasCredentials {
		return nil, nil
	}
	return &keychain{config: config}, nil
}

type keychain struct {
	config *configfile.ConfigFile
}

func (k *keychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	var config, empty dockertypes.AuthConfig
	for _, key := range []string{target.String(), target.RegistryStr()} {
		if key == name.DefaultRegistry {
			key = authn.DefaultAuthKey
		}
		var err error
		config, err = k.config.GetAuthConfig(key)
		if err != nil {
			return nil, err
		}
		config.ServerAddress = ""
		if config != empty {
			break
		}
	}
	if config == empty {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{
		Username:      config.Username,
		Password:      config.Password,
		IdentityToken: config.IdentityToken,
		RegistryToken: config.RegistryToken,
	}), nil
}
