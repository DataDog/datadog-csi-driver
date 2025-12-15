// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

// VolumeType represents the type of volume to mount.
type VolumeType string

const (
	// APMSocket mounts the APM socket file
	APMSocket VolumeType = "APMSocket"
	// APMSocketDirectory mounts the parent directory of the APM socket
	APMSocketDirectory VolumeType = "APMSocketDirectory"
	// DSDSocket mounts the DogStatsD socket file
	DSDSocket VolumeType = "DSDSocket"
	// DSDSocketDirectory mounts the parent directory of the DogStatsD socket
	DSDSocketDirectory VolumeType = "DSDSocketDirectory"
	// DatadogSocketsDirectory is deprecated, use DSDSocketDirectory instead
	DatadogSocketsDirectory VolumeType = "DatadogSocketsDirectory"
	// DatadogLibrary mounts a Datadog instrumentation library from an OCI image
	DatadogLibrary VolumeType = "DatadogLibrary"
	// DatadogInjectorPreload mounts the ld.so.preload file
	DatadogInjectorPreload VolumeType = "DatadogInjectorPreload"
)
