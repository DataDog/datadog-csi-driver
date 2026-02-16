# AI Agent Guidelines for datadog-csi-driver

This document provides guidelines for AI agents working on the datadog-csi-driver codebase.

## CHANGELOG Maintenance

**CRITICAL**: Always update `CHANGELOG.md` when making user-facing changes. This is non-negotiable.

### Format

The changelog follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). All entries go under the `## [Unreleased]` section using the appropriate subsection:

- **Added** — new features or capabilities
- **Changed** — changes to existing functionality
- **Deprecated** — features that will be removed in a future release
- **Removed** — features that were removed
- **Fixed** — bug fixes
- **Security** — vulnerability fixes or dependency upgrades for security reasons

### What to Document

**MUST include:**
- New volume types or publisher types
- New CLI flags or configuration options
- Bug fixes that affect driver behavior
- Breaking changes (flag renames, volume type changes, behavior changes)
- Security fixes and CVE remediations
- Changes to supported platforms or Kubernetes versions

**EXCLUDE:**
- Internal refactoring that doesn't change behavior
- Test improvements
- CI/CD pipeline changes
- Code style or linting changes
- Documentation-only updates

### When in Doubt

Ask: "Would a user deploying this CSI driver notice or care about this change?"
- **YES** → Update CHANGELOG.md under `## [Unreleased]`
- **NO** → Skip CHANGELOG.md

### Example Entry

```markdown
## [Unreleased]

### Added
- New `ExampleVolume` volume type for mounting example resources

### Fixed
- Fixed socket mount failing when host path contains symlinks
```

## Project Context

- **Language**: Go
- **Build**: Docker multi-arch (linux/amd64, linux/arm64)
- **Testing**: `go test ./...` for unit tests, `make e2e` for end-to-end tests
- **Versioning**: Semantic Versioning (semver). Tags use `v` prefix (e.g., `v1.2.0`)
- **License**: Apache-2.0. All Go source files must include the copyright header (see existing files for format).

