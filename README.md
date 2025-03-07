# Datadog CSI Driver <!-- omit in toc -->

This repository contains the source code for the Datadog CSI Driver.

This driver allows mounting `CSI` volumes instead of `hostPath` volumes in pods that need access for UDS sockets for DogStatsD and APM. This allows pods in namespaces with baseline or restrictive [pod security standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) to get access to the UDS sockets on which the datadog agent listens for custom metrics and APM traces.

## Features <!-- omit in toc -->

- **Mounting UDS sockets**: Supported using the `socket` mode.
- **Mounting local directories**: Supported using `local` mode.

## Table of Contents <!-- omit in toc -->

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [CSI Volume Structure](#csi-volume-structure)
    - [Socket Mode](#socket-mode)
    - [Local Mode](#local-mode)
- [License](#license)

## Getting Started

### Prerequisites
- Kubernetes 1.13 or later.
- Golang setup for development.
- Docker setup for building the driver.

### CSI Volume Structure

CSI volumes processed by this driver must have the following format:

```
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        mode: <mount-mode>
        path: <mount-path>
name: <volume-name>
```

Currently, two modes are supported:

#### Socket Mode

This mode is useful for mounting a UDS socket file.

The UDS socket file path should be specified in the `path` attribute and the mode should be set to `socket`.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        mode: socket
        path: /var/run/datadog/dsd.socket
name: datadog-dsd
```

In case the indicated `path` doesn't correspond to a UDS socket, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

#### Local Mode

This mode is useful for mounting a directory.

The directory path should be specified in the `path` attribute, and the mode should be set to `local`.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    readOnly: false
    volumeAttributes:
        mode: local
        path: /var/run/datadog
name: datadog
```

If the specified path is not found, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

## License

Distributed under the Apache License. See `LICENSE` for more information.
