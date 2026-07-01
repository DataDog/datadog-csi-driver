# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-07-01

### Added

- New `--registry-allow-list` flag (env: `DD_REGISTRY_ALLOW_LIST`) for `DatadogLibrary` volumes. When non-empty, only registries in the list are permitted; requests specifying an unlisted registry are rejected. Empty list (default) allows all registries.
- New Prometheus metrics for library lifecycle observability (the `library` label is the package name, e.g. `dd-lib-java-init`):
  - `datadog_csi_driver_library_resolutions_total{library,result}` (counter): library resolution outcomes (`cache_hit`, `downloaded`, `failed`).
  - `datadog_csi_driver_library_download_duration_seconds{library,registry}` (histogram): time spent downloading a library from the registry.
  - `datadog_csi_driver_library_cleanup_total{library,status,strategy}` (counter): cleanup attempts for unused libraries (`success`, `failed`, `skipped_in_use`).
  - `datadog_csi_driver_libraries_cached{library}` (gauge): number of versions currently stored on disk for each library.
  - `datadog_csi_driver_libraries_cached_bytes{library}` (gauge): cumulative on-disk size, in bytes, of cached versions for each library.
  - `datadog_csi_driver_library_volume_links{library}` (gauge): number of volumes currently linked to any cached version of a library.
- The on-disk bbolt database now stores, for each cached library, its package name, on-disk size and live volume reference count. Required to publish per-library gauges across restarts.

### Changed

- `librarymanager` no longer depends on `pkg/metrics`. Lifecycle events are reported through a new `libraryevents.Listener` interface defined in a dedicated, dependency-free `pkg/libraryevents` package; the metrics-publishing implementation lives in `pkg/metrics` and is wired in `pkg/driver`.
- `LinkVolume` now happens after the library has been confirmed on disk (cache hit) or recorded (successful download). This prevents dangling links if the download fails, and gives `library_volume_links` a stable library label.
- `RemoveVolume` is now a no-op when the volume was never linked.
- The on-disk bbolt schema was simplified to two flat buckets (`volumes`, `libraries`) replacing the previous nested `library-mappings`/`volume-mappings` buckets and the per-library metadata bucket. Existing databases are migrated in place on the first start after upgrade, preserving volume links and library metadata.

### Fixed

- Fixed `DatadogLibrary` volume publishing to reject library source paths that resolve outside the downloaded library directory.
- Fixed OCI archive extraction for `DatadogLibrary` volumes to prevent archive-planted symlinks from redirecting file writes outside the extraction directory.

### Fixed

- Fixed `DatadogLibrary` and `DatadogInjectorPreload` volume publishing to remount bind mounts as read-only.

### Notes

- Libraries cached on disk before this release have no metadata recorded; they are not counted in the `libraries_cached*` or `library_volume_links` gauges until they are downloaded again. The bias is expected to be short-lived because the library publisher is not yet in heavy use.

## [1.2.2] - 2026-04-21

### Fixed

- Fixed driver startup to disable SSI publishers when `storageBasePath` is not writable instead of failing initialization

## [1.2.1] - 2026-02-03

### Fixed

- Fixed library downloads pulling wrong CPU architecture on non-amd64 nodes

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

[Unreleased]: https://github.com/DataDog/datadog-csi-driver/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/DataDog/datadog-csi-driver/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/DataDog/datadog-csi-driver/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/DataDog/datadog-csi-driver/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/DataDog/datadog-csi-driver/compare/v1.0.1-beta...v1.0.0
[1.0.1-beta]: https://github.com/DataDog/datadog-csi-driver/compare/v0.0.2-beta...v1.0.1-beta
[0.0.2-beta]: https://github.com/DataDog/datadog-csi-driver/compare/v0.0.1-beta...v0.0.2-beta
[0.0.1-beta]: https://github.com/DataDog/datadog-csi-driver/releases/tag/v0.0.1-beta

