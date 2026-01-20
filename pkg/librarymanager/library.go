// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package librarymanager

import (
	"fmt"
)

// Library represents a Datadog package to download and mount as part of a DatadogLibrary volume request.
type Library struct {
	name     string
	registry string
	version  string
	pull     bool
}

// NewLibrary instatiates a new library from the provided fields and ensures they are valid.
func NewLibrary(name string, registry string, version string, pull bool) (*Library, error) {
	if name == "" {
		return nil, fmt.Errorf("name must be provided and cannot be empty")
	}
	if registry == "" {
		return nil, fmt.Errorf("registry must be provided and cannot be empty")
	}
	if version == "" {
		return nil, fmt.Errorf("version must be provided and cannot be empty")
	}

	return &Library{
		name:     name,
		registry: registry,
		version:  version,
		pull:     pull,
	}, nil
}

// Pull returns if this library should be pulled or not based on the pull policy.
func (l *Library) Pull() bool {
	return l.pull
}

// Image provides a container image path pullable by crane.
func (l *Library) Image() string {
	return fmt.Sprintf("%s/%s:%s", l.registry, l.name, l.version)
}
