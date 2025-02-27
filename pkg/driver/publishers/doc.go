/*
Package publishers contains the logic for publishing datadog CSI volumes depending on the volume mode.

Datadog CSI volume request must include `mode` and `path` properties in the `volumeAttributes`.

Currently, the only supported mode is `socket`.

The socket publisher processes Datadog CSI volume requests as follows:
- it first verifies that the requested `path` is indeed a UDS socket.
- if the specified path is not a socket, an error is returned.
- otherwise, it mounts the socket's parent directory onto the pod's target path.
*/
package publishers
