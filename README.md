# Datadog CSI Driver <!-- omit in toc -->

This repository contains the source code for the Datadog CSI Driver.

This driver allows mounting `CSI` volumes instead of `hostPath` volumes in pods that need access for UDS sockets for DogStatsD and APM. This allows pods in namespaces with baseline or restrictive [pod security standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) to get access to the UDS sockets on which the datadog agent listens for custom metrics and APM traces.

## Features <!-- omit in toc -->

- **Mounting Datadog dogstatsd socket**: Supported using the `DSDSocket` type.
- **Mounting Datadog trace agent socket**: Supported using `APMSocket` type.
- **Mounting Datadog agent sockets directory**: Supported using `DatadogSocketsDirectory` type.

## Table of Contents <!-- omit in toc -->

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [CSI Volume Structure](#csi-volume-structure)
    - [DSDSocket](#dsdsocket)
    - [APMSocket](#apmsocket)
    - [DatadogSocketsDirectory](#datadogsocketsdirectory)
- [License](#license)

## Getting Started

### Prerequisites
- Kubernetes 1.13 or later.
- Golang setup for development.
- Docker setup for building the driver.

### CSI Volume Structure

CSI volumes processed by this driver must have the following format:

```yaml
# maybe you can put this example with a Pod manifest context adding the full path

kind: Pod
metadata:
  name: foo
spec:
  containers:
    volumes:
      - csi:
          driver: k8s.csi.datadoghq.com
          volumeAttributes:
            type: <volume-type>
        name: <volume-name>
```

Currently, 3 types are supported:
* DSDSocket
* APMSocket
* DatadogSocketsDirectory

#### DSDSocket

This type is useful for mounting a dogstatsd UDS socket file.

For example:

```yaml
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        type: DSDSocket
name: datadog-dsd
```

In case the indicated socket doesn't exist, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

#### APMSocket

This type is useful for mounting a trace agent UDS socket file.

For example:

```yaml
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        type: APMSocket
name: datadog-apm
```

In case the indicated socket doesn't exist, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

#### DatadogSocketsDirectory

This mode is useful for mounting the directory containing the sockets of dogstatsd and apm.

For example:

```yaml
csi:
    driver: k8s.csi.datadoghq.com
    readOnly: false
    volumeAttributes:
        type: DatadogSocketsDirectory
name: datadog
```

## License

Distributed under the Apache License. See `LICENSE` for more information.
