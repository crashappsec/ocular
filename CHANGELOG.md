# Ocular Release Notes
<!-- https://keepachangelog.com -->

# [v0.2.6](https://github.com/crashappsec/ocular/releases/tag/v0.2.6) - **February 17th, 2026**

### Added
- Downloader and ClusterDownloader now support parameters
  - parameters are specified in the `downloaderRef` field of a pipeline
- Searches now have a "scheduler sidecar", which can create pipelines or searches just by writing to a FIFO
  - Searches has a new field `scheluder` where you can specify the pipeline template for created pipelines, and an internal for how long to space out newly created resources
  - writing to the file `$OCULAR_PIPELINE_FIFO` with the JSON of a target will result in the sidecar creating a pipeline based on the `pipelineTemplate` field of the search
  - writing to the file `$OCULAR_SEARCH_FIFO` with the JSON of a crawler reference will result in the sidecar creating a search with the same spec as the current search, but with the crawler reference updated.

### Fixes
- `ocular-extractor` now renamed `ocular-sidecar`
- Pipeline scheduler will now wait for the uploader's reciever to be running before creating scan pod

# [v0.2.5](https://github.com/crashappsec/ocular/releases/tag/v0.2.5) - **February 3rd, 2026**

### Added

- New Cluster wide resources for downloaders, uploaders, and crawlers

# [v0.2.4](https://github.com/crashappsec/ocular/releases/tag/v0.2.4) - **January 15, 2026**

### Added

- Custom application specific metrics now published via the kubernetes controller metrics endpoint
  - `ocular_pipelines_completed_total` - Counter of total number of pipelines created
  - `ocular_pipelines_running` - Gauge for the number of pipelines currently running
  - `ocular_scan_pods_created_total` - Counter for the total number of scan pods created
  - `ocular_upload_pods_created_total` - Counter for the total number of upload pods created
  - `oulcar_pipeline_duration_seconds` - Summary for the number of seconds a pipeline took to complete
  - `ocular_search_pods_created_total` - Counter for the total number of search pods created
  - `ocular_search_duration_seconds` - Summary for the number of seconds a search took to complete

# [v0.2.3](https://github.com/crashappsec/ocular/releases/tag/v0.2.3) - **December 10, 2025**

### Added

- New `Pipeline.Status.Phase` field to indicate the current phase of the pipeline (e.g., Pending, Downloading, Scanning, Uploading, Completed, Failed)
- New `Pipeline.Status.StageStatuses` field to provide detailed status information for each stage of the pipeline (downloader, scanner, uploader)

### Fixes

- Fixed panic when HTTP requests failed in the extractor


# [v0.2.2](https://github.com/crashappsec/ocular/releases/tag/v0.2.2) - **November 17, 2025**

### Added

- Support for downloaders to pass metadata files about target to scanners/uploaders
    - files written to `/mnt/metadata` and specified in the Downloader spec will now be combined with the scanner artifacts and passed to uploaders

- Ability for searches to specify a custom service account instead randomly generating one per search
    - if a custom search account is specified, a temporary role-binding is given to give the service account permissions to start pipelines/searches in the namespace where the search was created
    - a custom service account will not have it owner references set

- Addition of `additionalPodMetadata` field for pipelines/searches where user can specify additional annotations and labels to add to the child pods of the resource

### Changes

- improve use of kustomize deployment, can now run `make deploy-$NAME` where `$NAME` is a folder in the `config/` directory to deploy, instead of `default` (which is used by `make deploy`
- use `omitzero` on fields on CRDs that when not specified were rendering as `{}`


# [v0.2.1](https://github.com/crashappsec/ocular/releases/tag/v0.2.1) - **October 23, 2025**

### Added
- `TTLSecondsMaxLifetime` for Pipeline resources
    - A non-zero value `N` means they will be deleted `N` seconds after creation
- Publish separate image for the extractor
- Helm Chart now generates with support for customizing deployment of controller
  - Can add custom labels, environment variables, and volume/volume mounts

### Changed
- Use of pods over jobs for execution resources
- Bump golangci lint version in CI

### Fixed
- Zap logger is set to production by default
- `LDFLAGS` in Makefile to set metadata for dev builds


# [v0.2.0](https://github.com/crashappsec/ocular/releases/tag/v0.2.0) - **September 29, 2025**

### Added

- Support for custom resource definitions (CRDs) to define uploaders, downloaders, crawlers, profiles, and secrets.
- Use of a Kubernetes controller to manage the lifecycle of scanning jobs.
- Enhanced API endpoints for managing pipelines, searches, and resources.
- Improved logging and monitoring capabilities.

### Changed
- Refactored the API server to work seamlessly with the Kubernetes controller and CRDs.
- Updated documentation to reflect the new architecture and usage patterns.
- Use of domain `ocular.crashoverride.run` for all annotations, and resource API group.

### Deprecated

- Deprecated the standalone API server in favor of a Kubernetes-native approach.
- Deprecated old resource definitions in favor of CRDs.

# [v0.1.1](https://github.com/crashappsec/ocular/releases/tag/v0.1.1) - **July 16, 2025**

### Added

- Ability to customize Kubernetes annotations for jobs and pods.
- Support for setting custom labels and annotations in development environment.

### Changed

- Default labels and annotations for jobs and pods.
  - use domain `ocularproject.io` for all annotations.

# [v0.1.0](https://github.com/crashappsec/ocular/releases/tag/v0.1.0) - **July 15, 2025**

### Added

- Initial release of Ocular, a Kubernetes-native API for security scanning of static software assets.
- Support for defining uploaders, downloaders, and crawlers as resources as well as profiles and secrets.
- Basic RESTful API for managing pipelines, searches and resources.
- Setup development workflows, contributing guidelines, and documentation.
