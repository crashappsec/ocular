# Profiles

A Profile is a collection of scanners (container images), a list of artifacts that the scanner produce, and a list of 
uploader names that will be used to upload the artifacts produced by the scanners. A profile will be executed as part of a pipeline,
where a target is downloaded to the current working directory, the scanners are executed in parallel followed by the uploaders.
Uploaders are defined separately and can be reused across different profiles, see [UPLOADERS.md](/docs/definitions/UPLOADERS.md) for more information on how to define an uploader.
For more information on how to execute a profile, see the [pipeline documentation](/docs/executions/PIPELINES.md).
Profiles are intended to separate the types of scans and allow for triggering different sets of scans based on the target type,
target source, or other criteria.


## Definition

A profile has 3 components:
- **Scanners**: A list of container images that will be executed in parallel to scan the target content. This has
the same definition as a [User Container](/docs/definitions/CONTAINERS.md#usercontainer-with-parameters) definition
- **Artifacts**: A list of artifacts that the scanners will produce. Each artifact is a file path relative to the 'results' directory in the container filesystem.
The path to the results directory is provided as the `OCULAR_RESULTS_DIR` environment variable.
- **Uploaders**: A list of uploader names and parameters that will be used to upload the artifacts produced by the scanners.
See [UPLOADERS.md](/docs/definitions/UPLOADERS.md) for more information on how to define an uploader.

```yaml
# Example definition of a profile.
# It can be set by calling the endpoint `POST /api/v1/profiles/{name}`
# with the body containing the definition in YAML or JSON format (depending on Content-Type header).
# The name of the profile is set by the `name` path parameter and is used to identify the profile in the system.

# Each item of the scanners list is a User Container definition.
# See /docs/definitions/CONTAINERS.md for the full list of options and schema definition.
# An example is provided below and does not include all options.
scanners:
  - image: "myorg.domain/scanner:latest"
    imagePullPolicy: IfNotPresent
    command: ["/bin/sh", "-c"]
    args:
      - "python3 --verbose scanner.py --results-dir=$OCULAR_RESULTS_DIR/report.json"
    env:
      - name: LOG_LEVEL
        value: "debug"
    secrets:
      - name: token
        mountType: envVar
        mountTarget: MY_TOKEN
  - image: "myorg.domain/another-scanner:latest"
    imagePullPolicy: IfNotPresent
    command: ["/bin/bash", "-c"]
    args:
      - "./my-scanner.sh --output=$OCULAR_RESULTS_DIR/output.txt"
    secrets:
      - name: my-config
        mountType: file
        mountTarget: /etc/config.yaml
# List of artifacts that the scanners will produce.
# These are file paths relative to the 'results' directory in the container filesystem.
# The path to the results directory is provided as the `OCULAR_RESULTS_DIR` environment variable.
artifacts:
  - "report.json"
  - "output.txt"
# List of uploaders to use for uploading the artifacts produced by the scanners.
# Each item is a map with the uploader name and any parameters that should be passed to the uploader.
# The uploader must be defined in the system, or the profile will fail to be created.
# Additionally, All required parameters for the uploader must be provided or the profile will fail to be created.
# To view the parameters that can be passed to an uploader, check the definitions from the endpoint `/api/v1/uploaders/{uploader_name}`
# for the specific uploader.
uploaders:
  - name: "my-uploader"
    parameters:
      PARAM1: "value1"
      PARAM2: "value2"
  - name: "another-uploader"
    parameters:
      CUSTOM_PARAM: "custom_value"
```