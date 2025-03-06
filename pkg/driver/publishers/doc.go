/*
Package publishers contains the logic for publishing datadog CSI volumes depending on the volume mode.

Datadog CSI volume request must include `mode` and `path` properties in the `volumeAttributes`.

Supported publisher modes are:
- socket: mounts UDS sockets.
- local: mounts existing directories.
*/
package publishers
