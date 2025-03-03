/*
Package publishers contains the logic for publishing datadog CSI volumes depending on the volume mode.

Datadog CSI volume request must include `mode` and `path` properties in the `volumeAttributes`.

Currently, the only supported mode is `socket`.

The socket publisher processes Datadog CSI volume requests by verifying that the requested path is
indeed a socket path. If it is verified, the socket is mounted at the pod's target path. An error is
returned otherwise.
*/
package publishers
