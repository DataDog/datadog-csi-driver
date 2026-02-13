# Release Process

## Steps

1. Checkout the repository on the correct branch (`main`)

2. Update the changelog — move all entries from `## [Unreleased]` into a new version section:
    ```markdown
    ## [X.Y.Z] - YYYY-MM-DD
    ```
    Update the comparison links at the bottom of `CHANGELOG.md`:
    ```markdown
    [Unreleased]: https://github.com/DataDog/datadog-csi-driver/compare/vX.Y.Z...HEAD
    [X.Y.Z]: https://github.com/DataDog/datadog-csi-driver/compare/vPREVIOUS...vX.Y.Z
    ```

3. Commit the changelog update:
    ```console
    $ git add CHANGELOG.md
    $ git commit -m "Release vX.Y.Z"
    ```

4. Add the release tag:
    ```console
    $ git tag vX.Y.Z
    ```

5. Push the commit and tag:
    ```console
    $ git push origin main
    $ git push origin vX.Y.Z
    ```

6. The GitLab pipeline is triggered. Manually trigger the publish jobs to release the new version to container registries.

## Versioning

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (`X`): Incompatible changes (e.g., removing a volume type, renaming flags without backward compatibility)
- **MINOR** (`Y`): New features in a backward-compatible manner (e.g., new volume types, new flags)
- **PATCH** (`Z`): Backward-compatible bug fixes and security patches

## Changelog

All user-facing changes must be documented in `CHANGELOG.md` before release. See [CHANGELOG.md](CHANGELOG.md) for the format and [AGENTS.md](AGENTS.md) for guidelines on what to include.
