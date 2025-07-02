# Crawlers

Crawlers are container images (defined by the user) that are used to enumerate targets to scan.
The container is expected to gather a set of targets to scan, then call the API to start scans for those targets.
The container will be given an authenticated token to the API, allowing them to call the API to start scans or other crawlers.
The crawlers can be run on a schedule or on demand, and can be configured to pass a set of parameters when invoked.

## Definition

A Crawler definition is the same as a [User Container With Parameters](/docs/definitions/CONTAINERS.md#usercontainer) definition.
See [CONTAINERS.md](/docs/definitions/CONTAINERS.md) for the full list of options and schema definition.
The example below is provided for reference and does not include all options.

```yaml
# Example definition of a crawler.
# It can be set by calling the endpoint `POST /api/v1/crawlers/{name}`
# with the body containing the definition in YAML or JSON format (depending on Content-Type header).
# The name of the crawler is set by the `name` path parameter and is used to identify the crawler in the system.

# Container Image URI 
image:  "myorg.domain/crawler:latest"
# Pull policy for the container image.
imagePullPolicy: IfNotPresent
# Command and args to execute when the crawler is run.
command: ["python3", "crawler.py"]
args:
  - "--verbose"
  - "--scan-folder=./"
# Set environment variables for the crawler.
env:
  - name: LOG_LEVEL
    value: "debug"
# Secrets to mount when the crawler is executed.
secrets:
  - id: crawler-github-token
    mountType: envVar
    mountTarget: GITHUB_TOKEN
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