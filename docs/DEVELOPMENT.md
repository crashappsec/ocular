# Development

This document describes the development process for the project.

## Getting Started

This guide is a basic overview of how to get started with the project
when developing locally. We recommend you first read the [documentation site](https://ocularproject.io/docs/)
to understand the project and how to use the API.

### Prerequisites

- [Go](https://golang.org/doc/install/source) 1.24 or later
- [Docker](https://docs.docker.com/get-docker/)
- [Kubernetes](https://kubernetes.io/docs/home/) 1.30 or later
    - Additionally, you should have a working `kubectl` installation and a configured kubeconfig file.
    - You should have a service account ~~or user~~ with the following permissions:
        - To Install: `create` on `namespaces`, `role`, `role-binding`, `serviceaccounts`, `deployments`, `pods`,
          `configmaps` and `secrets`
        - To Run: `get`, `list`, `watch`, `create` on `jobs.batch`, `configmaps`, `secrets` on a service account
- Reasonable standard CLI development tools:
    - `make`
    - `curl` or `httpie` (or any other HTTP client)
    - `jq`
    - `git` (for downloading git repositories)
    - `kubectl` (for interacting with kubernetes)

### Configuration

When running the application in production, it is recommended to use the use configuration
provided by the chart in the [`helm-charts`](https://github.com/crashappsec/helm-chart) repository.
The chart can also be viewed on [Artifact Hub](https://artifacthub.io/packages/helm/crashoverride-helm-charts/ocular).

Developers are encouraged to use `make` commands to run the application and manage the development environment.
Many of the scripts used during development will rely on environment variables to configure settings, which 
are loaded from a `.env` automatically by the `make` command. An example `.env` file is provided in the repository
as [`example.env`](/example.env). This file contains documentation for the available environment variables. 
To get started, copy the `example.env` file to `.env` and then edit it to configure the application.

**NOTE**: Make can also be configured to use a different environment file by setting the `OCULAR_ENV_FILE` variable in the Makefile.

```bash
cp example.env .env
# edit the .env file to configure the application

# OR #
cp example.env .env.staging # a specific environment file
export OCULAR_ENV_FILE=.env.staging # set the environment file for make to use
```

### Running the Application

the API requires a few resources in the kubernetes cluster to run. A development
version of the resources can be created in your cluster by running the following command:

```bash
# Create local dev resources in local kubernetes cluster.
# NOTE: this will use the current context in your kubeconfig
# the kubeconfig path will be read from the env variable KUBECONFIG
# or ~/.kube/config if not set
make apply-devenv
# you can undo this with make remove-devenv
```

#### Installing default integrations

When installing via Helm, the [default integrations](https://github.com/crashappsec/ocular-default-integrations)
(downloaders, scanners, uploaders) are installed automatically.

When running the application locally, you can install the default integrations by running the following command:

```bash
# Install the default integrations to the development environment.
# You can set the OCULAR_DEFAULTS_VERSION environment variable to
# specify a specific version of the defaults to install.
# defaults to the latest version if not set.
make apply-devenv-default
# or: OCULAR_DEFAULTS_VERSION=0.1.0 make apply-devenv-defaults
```


The API can then either be run via docker or locally as a go application:

```bash
# Run the application via docker
# The API will read your kubeconfig in order to get access the cluster.
# the kubeconfig path will be read from the env variable KUBECONFIG
# or ~/.kube/config if not set

make run-docker-local
# or 
# make run-local # to run the application locally as a binary

# Once running you can access the application at http://localhost:3001
# NOTE: docker will need to use --network=host to access the kubernetes cluster
# so host networking is required
```

### Accessing the API

The API uses Bearer tokens for authentication. `kubectl` can be used to
create a token for the service account that is used to run the API. The service account
should be created by the `devenv-up` command.

```bash
# Runs 'kubectl create-token' for the service account
make generate-devenv-token
```

### Creating a profile

The API uses profiles to define the configuration for both the scanning tools AND
the uploading of any artifacts or results from the scans.
A profile is a set of configuration options that are used to run a scan.
A profile can be created by sending a POST request to the `/profiles` endpoint

```bash
# This creates a profile named 'gitleaks' which runs the gitleaks
# scanner and prints results to the STDOUT of the container
# see the other example profiles in the hack/profiles directory
curl -L -X POST -H "Authorization: Bearer $(make generate-devenv-token)" \
  -H "Content-Type: application/x-yaml" \
  --data-binary '@hack/profiles/gitleaks-example.yaml' \
  http://localhost:3001/api/v1/profiles
````

### Configuring Secrets

Secrets can be set via the API by sending a POST request to the `/secrets` endpoint.
NOTE: secrets cannot be retrieved via the API, they can only be created and deleted.

```bash
# This creates a secret named 'downloader-gitconfig' from the file
# my/example/gitconfig
curl -L -X POST -H "Authorization: Bearer $(make generate-devenv-token)" \
  --data-binary '@my/example/gitconfig' \
  http://localhost:3001/api/v1/secrets/downloader-gitconfig
```

Secrets can be referenced in the profile configuration by using the `secrets` field in
a scanner definition. An example is shown below

```yaml
# Example profile
name: my-example-profile

# This section defines the scanners that will be used to scan the target
# once the target is downloaded (via the downloader set when creating the pipeline)
scanners:
  - image: "my-scanner-image"
    command: [ "/my-scanner" ]
    args: [ "--fast", "--json", "--output", "$RESULTS_DIR/output.json" ]
    secrets:
      - id: file-mounted-secret # this is the name of the secret created via the API
        mountType: "file" # this specifies how to mount the secret (file or envVar)
        mountTarget: "test.txt" # this is the path to mount the secret to (for file mounts)
        required: true # this specifies if the secret is required or not (default: false)
      - id: envvar-mounted-secret # this is the name of the secret created via the API
        mountType: "envVar" # This specifies an environment variable mount
        mountTarget: "MY_SECRET" # this is the name of the environment variable
  - image: "my-other-scanner"
    command: [ '/another-scanner' ]
    args: [ "--results", "$RESULTS_DIR/other-output.json" ]
# This section defines the artifacts (output files) that will be created by the scanners
# They should all be located within the $RESULTS_DIR directory (i.e. /mnt/results).
# any non-absolute paths will be relative to this directory
artifacts:
  - output.json # configure the results from "my-scanner-image"
  - other-output.json # configure the results from "my-other-scanner"
# This section defines the uploaders that will be used to upload the results
# These are docker images that will be run in the 'results'. The uploaders
# are configured via the API endpoint /api/v1/uploaders (see below)
uploaders:
  # This will map to the name of a configured uploader
  - name: "webhook"
    # "webhook" is a pre-configured uploader that will send the results via an HTTP
    # webhook as the body
    parameters:
      METHOD: PUT
      URL: https://my.webhook.url/path
```

### Configuring Downloaders

A "downloader" is a tool that is used to download a static assest to be scanned.
Users will be able to configured custom downloaders via the API, but the API also
contains a set of pre-configured downloaders that can be used to download static assets

- See the [downloaders documentation](/docs/definitions/DOWNLOADERS.md) for more information on how to define a downloader.
- See the [default downloaders documentation](/docs/definitions/DEFAULTS.md#downloaders) for a list of pre-configured downloaders.


### Running a scan

The API can be used to run a scan by sending a POST request to the `/pipeline` endpoint.
The request should contain the profile name and the target to scan.
A target contains the name of the downloader to use, the identifier for the target and an optional version.
In this example we are using the `gitleaks` profile (created in the example above) to scan a GitHub repository.
We tell the API to use the `git` downloader to download the repository, one of the pre-configured downloaders in the API.

```bash
# This runs a scan using the gitleaks profile.
# since we are using the 'git' downloader, we set the identifier to the clone URL
# and omit the version, which will default to the default branch.
curl -L -X POST -H "Authorization: Bearer $(make generate-devenv-token)" \
  -H "Content-Type: application/json" \
  -d '{"profileName": "gitleaks", "target": {"downloader": "git", "identifier": "https://github.com/crashappsec/chalk"}}' \
  http://localhost:3001/api/v1/pipelines
```

### Stopping a scan

The API can be used to stop a scan by sending a DELETE request to the `/pipeline/:id` endpoint
where `:id` is the id of the pipeline to stop.

```bash
# This stops the pipeline with id '1234'
curl -L -X Delete -H "Authorization: Bearer $(make generate-devenv-token)" \
  http://localhost:3001/api/v1/pipelines/1234
```

for more information on how to configure Ocular and API usage, please refer to the [Ocular manual](https://ocularproject.io/docs/manual).

### OpenAPI & Swagger UI

When running the application in development mode, the OpenAPI specification is available at `/api/swagger/openapi.json`.
This can be set by setting the `OCULAR_ENV` environment variable to `development` in the `.env` file.
The Swagger UI is available at `/api/swagger/`.

