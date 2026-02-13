# Versioning 

Ocular will follow semantic versioning (https://semver.org/), with `v0.0.0` as the initial version. The versioning scheme will be as follows:
- **Major Version**: Incremented for incompatible API changes. "API" will include both the REST API but additionally any changes to the runtime
  that would require a change in a client image (downloaders, crawlers, scanners, etc.).
- **Minor Version**: Incremented for new features that are backward compatible. This includes new endpoints, new functionality in existing endpoints, or
  additional capabilities in the runtime that do not break existing clients.
- **Patch Version**: Incremented for backward-compatible bug fixes. This includes fixes to existing endpoints, runtime issues, or any other changes that do not
  affect the API or functionality in a way that would require a change in client images.

## Versioning Scheme

The versioning scheme will be as follows:
```goregexp
v(?<major>\d+)\.(?<minor>\d+)\.(?<patch>\d+)(?:-(alpha|beta|rc)\.(?<build>\d+))?
```

## Versioning Examples

- `v0.0.0`: Initial version.
- `v1.0.0`: First stable release.
- `v1.1.0`: New feature added, backward compatible.
- `v1.1.1`: Bug fix, backward compatible.
- `v2.0.0`: Major version change, incompatible API changes.
- `v2.1.0-alpha.1`: Pre-release version, new feature added, backward compatible.
- `v2.1.0-beta.1`: Pre-release version, new feature added, backward compatible.

## Docker Tagging

Docker images will be tagged using the same versioning scheme as the application. For example:
- `ocular-controller:v1.0.0`: release v1.0.0
- `ocular-controller:latest`: latest stable release
- `ocular-controller:v1.1.0-beta.1`: pre-release version v1.1.0-beta.1
- `ocular-controller:latest-prerelease`: latest pre-release version

## Releases

Releases are managed via GitHub releases.
Each release will be tagged with the corresponding version number and will include release notes detailing the changes made in that version.

Each GitHub release will publish both the controller and sidecar images to GitHub Container Registry (GHCR) with the appropriate tags,
and upload a manifest file to the release (named `ocular.yaml`) that can be installed via `kubectl apply -f <URL>`.

Latest releases will be tagged from the `main` branch, while pre-releases (alpha, beta, rc) will be tagged from a development branch,
typically with the name `feat/<release>` before merge to `main`.