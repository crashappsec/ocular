# Uploaders

Uploaders are container images (defined by the user) that are used to process or upload data to a another system.
The container images are expected to read the files to upload from the paths given to them via command line arguments
and then perform the upload operation on those files. Uploaders will be defined via the API and then can be ran by
referencing them in a profile definition (see the [profile documentation](/docs/definitions/PROFILES.md)
for more details on how to define a profile). 

## Definition

An Uploader definition is the same as a [User Container with Parameters](/docs/definitions/CONTAINERS.md#user-container-with-parameters) definition.
See [CONTAINERS.md](/docs/definitions/CONTAINERS.md) for the full list of options and schema definition.
The example below is provided for reference and does not include all options.

```yaml
# Example definition of an uploader.
# It can be set by calling the endpoint `POST /api/v1/uploaders/{name}`
# with the body containing the definition in YAML or JSON format (depending on Content-Type header).
# The name of the uploader is set by the `name` path parameter and is used to identify the uploader in the system.

# Container Image URI 
image:  "myorg.domain/uplodaer:latest"
# Pull policy for the container image.
imagePullPolicy: IfNotPresent
# Command and args to execute when the crawler is run.
command: ["ruby", "uploader.rb"]
args:
  - "--quiet"
  - "-f"
  - "./"
# Set environment variables for the crawler.
env:
  - name: AWS_CONFIG
    value: "/creds/aws-config"
# Secrets to mount when the crawler is executed.
secrets:
  - id: uploader-aws-config
    mountType: file
    mountTarget: "/creds/aws-config"
# Parameter definitions for the crawler.
# These are provided when the crawler is invoked,
# or scheduled to run.
parameters:
  DOWNLOADER:
    description: The name of the downloader to use for created pipelines
    required: false
    default: "git"
  GITHUB_ORGS:
    description: The name of the GitHub organization to crawl.
    required: true
  PROFILE_NAME:
    description: The name of the profile to use for created pipelines
    required: true
  SLEEP_DURATION:
    description: How long to sleep in between pipeline creations (e.g. 1m30s)
    required: false
    default: "1m30s"
```