# Pipelines

Pipelines are the core of the Ocular system.
They are used to download target content, run scanners on it, and upload the results to 3rd party systems.
When triggering a pipeline, the user will provide a target identifier (e.g. a URL or a git repository), an optional target version, the name of the downloader to use, and a profile to run.
The pipeline will then execute the following steps:

1. **Download**: The pipeline will run the specified downloader with the target identifier and version set as environment variables.
   The downloader is expected to download the target content to its current working directory. Once the container exits (with code 0), the pipeline will proceed to the next step.
2. **Scan**: The pipeline will run the scanners specified by the provided profile, which are run in parallel.
   Each scanner will be executed in its own container (but still on the same pod), with the current working directory set to the directory where the downloader wrote the target content.
   The scanners should produce artifacts, and send them to the `artifacts` directory in the container filesystem (the path is given as the `OCULAR_ARTIFACTS_DIR` environment variable).
3. **Upload**: Once all scanners have completed, the pipeline will extract the artifacts (listed in the profile)
   and run the uploaders in parallel.
   The uploaders will be passed the paths of the artifacts produced by the scanners as command line arguments.
   The uploaders are expected to upload the artifacts to a specific location (e.g. a database, cloud storage, etc.).
4. **Complete**: Once all uploaders have completed, the pipeline will be considered complete.

Currently, there is no feedback mechanism for the pipeline execution, so the user will need to check the API status of the pipeline execution to see if it was successful or not.

## Definitions

### Request

A pipeline request is a simple YAML or JSON object that contains the following fields:

```yaml
# Example pipeline request
# It can be sent to the endpoint `POST /api/v1/pipelines`
# with the body containing the request in YAML or JSON format (depending on Content-Type header).

# Target identifier (e.g. a URL or a git repository)
# It is up to the downloader to interpret this identifier and download the target content.
target: 
    identifier: "http://github.com/myorg/myrepo"
    downloader: "my-custom-git-downloader" # Name of the downloader to use for this pipeline
    # version: "v1.0.0" # Optional version of the target, if applicable
profileName: "my-custom-profile" # Name of the profile to use for this pipeline
```

### Response

The response to a pipeline request is the state of the pipeline execution. This can be queried via the endpoint `GET /api/v1/pipelines/{pipeline_id}`.

```yaml
id: "12345678-1234-1234-1234-123456789012" # Unique identifier of the pipeline execution
state: "Pending" # Current state of the pipeline execution (e.g. Pending, Running, Completed, Failed)
# The state will start as "Pending" and will change to "Running" when the downloader container begins its execution.
profile: "my-custom-profile" # Name of the profile used for this pipeline
target:
  identifier: "http://github.com/myorg/myrepo"
  downloader: "my-custom-git-downloader" # Name of the downloader to use for this pipeline
  # version: "v1.0.0" # Optional version of the target, if was provided
```