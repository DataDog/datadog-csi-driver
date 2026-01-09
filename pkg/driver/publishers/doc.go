// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

/*
Package publishers contains the logic for publishing Datadog CSI volumes depending on the volume type.

Datadog CSI volume requests should include a `type` property in the `volumeAttributes`.

Supported volume types are:
  - APMSocket: mounts the APM UDS socket.
  - DSDSocket: mounts the DogStatsD UDS socket.
  - APMSocketDirectory: mounts the directory containing the APM socket.
  - DSDSocketDirectory: mounts the directory containing the DogStatsD socket.
  - DatadogSocketsDirectory: mounts the directory containing both sockets.

Deprecated: The legacy `mode`/`path` schema is still supported but deprecated.
Please migrate to using the `type` schema instead.
*/
package publishers
