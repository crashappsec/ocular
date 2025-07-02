# Downloaders

Downloaders are container images (defined by the user) that are used to write a target to disk for scanning.
The container images are expected to read the target identifier and optional identifier version from environment variables
and then write the target to the current directory. Downloaders will be defined via the API and then can be ran by
referencing them when creating a new pipeline (see the [pipeline documentation](/docs/executions/PIPELINES.md)
for more details on how to execute a pipeline).

## Definition

A Downloader definitions is the same as a [User Container](/docs/definitions/CONTAINERS.md#user-container) definition.
See [CONTAINERS.md](/docs/definitions/CONTAINERS.md) for the full list of options and schema definition.
The example below is provided for reference and does not include all options.

```yaml
# Example definition of a downloader.
# It can be set by calling the endpoint `POST /api/v1/downloaders/{name}`
# with the body containing the definition in YAML or JSON format (depending on Content-Type header).
# The name of the downloader is set by the `name` path parameter and is used to identify the uploader in the system.

# Container Image URI 
image:  "myorg.domain/downloader:latest"
# Pull policy for the container image.
imagePullPolicy: Always
# Command and args to execute when the crawler is run.
command: ["./download"]
args:
  - "--path"
  - "."
# Set environment variables for the crawler.
env:
  - name: GIT_CONFIG
    value: "/creds/aws-config"
# Secrets to mount when the downloader is executed.
secrets:
  - id: downloader-github-token
    mountType: envVar
    mountTarget: GITHUB_TOKEN
```