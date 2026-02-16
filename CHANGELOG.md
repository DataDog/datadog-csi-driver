# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-02-03

### Added

- [Preview] SSI (Single Step Instrumentation) support: new `DatadogLibrary` volume type for mounting APM libraries from OCI images.

### Changed

- Migrated logging from klog to standard library `slog` with JSON format

## [1.1.1] - 2026-01-13

### Fixed

- Bumped Go version to fix CVE-2024-61729

## [1.1.0] - 2025-11-03

### Fixed

- Upgraded `golang.org/x/net` to resolve CVE-2025-22872 and CVE-2025-22870
- Bumped Go version for security fixes

## [1.0.0] - 2025-07-16

### Added

- First stable release

## [1.0.1-beta] - 2025-06-27

### Changed

- Internal build configuration updates

## [0.0.2-beta] - 2025-06-16

### Added

- E2e tests running in CI
- Support for deprecated `DatadogSocketsDirectory` volume type for backward compatibility

### Fixed

- Restored backward compatibility with previous volume type names

## [0.0.1-beta] - 2025-04-25

### Added

- Initial beta release
- CSI Node driver for mounting Datadog agent sockets
- `APMSocket` volume type for mounting APM trace agent UDS socket
- `APMSocketDirectory` volume type for mounting APM socket directory
- `DSDSocket` volume type for mounting DogStatsD UDS socket
- `DSDSocketDirectory` volume type for mounting DogStatsD socket directory
- Prometheus metrics endpoint for basic telemetry
- Multi-architecture Docker image support (linux/amd64, linux/arm64)

[Unreleased]: https://github.com/DataDog/datadog-csi-driver/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/DataDog/datadog-csi-driver/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.0.1-beta...v1.0.0
[1.0.1-beta]: https://github.com/DataDog/datadog-csi-driver/compare/v0.0.2-beta...v1.0.1-beta
[0.0.2-beta]: https://github.com/DataDog/datadog-csi-driver/compare/v0.0.1-beta...v0.0.2-beta
[0.0.1-beta]: https://github.com/DataDog/datadog-csi-driver/releases/tag/v0.0.1-beta

