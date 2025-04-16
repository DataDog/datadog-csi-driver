# Datadog CSI Driver <!-- omit in toc -->

This repository contains the source code for the Datadog CSI Driver.

This driver allows mounting `CSI` volumes instead of `hostPath` volumes in pods that need access for UDS sockets for DogStatsD and APM. This allows pods in namespaces with baseline or restrictive [pod security standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) to get access to the UDS sockets on which the datadog agent listens for custom metrics and APM traces.

## Features <!-- omit in toc -->

- **Mounting Datadog dogstatsd socket**: Supported using the `DSDSocket` or `DSDSocketDirectory` volume type.
- **Mounting Datadog trace agent socket**: Supported using `APMSocket` or `APMSocketDirectory` volume type.

## Table of Contents <!-- omit in toc -->

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [CSI Volume Structure](#csi-volume-structure)
    - [APMSocket](#apmsocket)
    - [APMSocketDirectory](#apmsocketdirectory)
    - [DSDSocket](#dsdsocket)
    - [DSDSocketDirectory](#dsdsocketdirectory)
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
        type: <volume-type>
name: <volume-name>
```

For example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-name
spec:
  containers:
    - name: ubuntu
      image: ubuntu
      command: ["/bin/bash", "-c", "--"]
      args: ["while true; do sleep 30; echo hello-ubuntu; done;"]
      volumeMounts:
        - mountPath: /var/sockets/apm/
          name: dd-csi-volume-apm-dir
        - mountPath: /var/sockets/dsd/dsd.sock
          name: dd-csi-volume-dsd
  volumes:
    - name: dd-csi-volume-dsd
      csi:
        driver: k8s.csi.datadoghq.com
        volumeAttributes:
          type: DSDSocket
    - name: dd-csi-volume-apm-dir
      csi:
        driver: k8s.csi.datadoghq.com
        volumeAttributes:
          type: APMSocketDirectory
```

Currently, 4 types are supported:
* APMSocket
* APMSocketDirectory
* DSDSocket
* DSDSocketDirectory

#### APMSocket

This type is useful for mounting a trace agent UDS socket file.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        type: APMSocket
name: datadog-apm
```

In case the indicated socket doesn't exist, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

#### APMSocketDirectory

This mode is useful for mounting the directory containing the apm socket.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    readOnly: false
    volumeAttributes:
        type: APMSocketDirectory
name: datadog
```

#### DSDSocket

This type is useful for mounting a dogstatsd UDS socket file.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    volumeAttributes:
        type: DSDSocket
name: datadog-dsd
```

In case the indicated socket doesn't exist, the mount operation will fail, and the pod will be blocked in `ContainerCreating` phase.

#### DSDSocketDirectory

This mode is useful for mounting the directory containing the dogstatsd socket.

For example:

```
csi:
    driver: k8s.csi.datadoghq.com
    readOnly: false
    volumeAttributes:
        type: DSDSocketDirectory
name: datadog
```

## License

Distributed under the Apache License. See `LICENSE` for more information.
