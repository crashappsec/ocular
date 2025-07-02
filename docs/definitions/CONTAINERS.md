# Container Definitions

The system allows customization of many aspects of the application through container definitions.
The container definition template is shared between many resources 
and is used to define the container image to run, the pull policy, and any secrets that should be mounted into the container.
In order to not duplicate documentation, the container definition is documented below.
There exists 2 main types of container definitions in the system:
- **User Container**: A standard container definition that is used to define a container image to run.
- **User Container With Parameters**: A container definition that is used to define a container image to run, along with a set of parameters that can be passed to the container when it is invoked.
It is a superset of the UserContainer definition, and adds a set of parameters that can be passed to the container when it is invoked.


The following resources use the User Container definition:
- [**Downloaders**](/docs/definitions/DOWNLOADERS.md): Used to define a container image that will download the target content to a specific directory in the container filesystem.
- [**Scanner (a subset of Profile)**](/docs/definitions/PROFILES.md): Used to define a container image that will scan the target content and produce artifacts.

The following resources use the User Container With Parameters definition:
- [**Uploaders**](/docs/definitions/UPLOADERS.md): Used to define a container image that will upload the artifacts produced by the scanners to a specific location.
- [**Crawlers**](/docs/definitions/CRAWLERS.md): Used to define a container image that will enumerate targets to scan and call the API to start scans for those targets.


## Definitions

### User Container

```yaml
# Example definition of a user container.

# Container Image URI
image:  "myorg.domain/crawler:latest"
# Pull policy for the container image.
# See https://kubernetes.io/docs/concepts/containers/images/#updating-images
imagePullPolicy: IfNotPresent
# Command and args to execute when the container is run.
# See https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#define-a-command-and-arguments-when-you-create-a-pod
command: ["python3", "crawler.py"]
args:
  - "--verbose"
  - "--folder=./"
# Set environment variables for the container.
# These are set in the container environment when the container is run.
# NOTE: Environment variables should not contain sensitive information.
# instead, use secrets to mount sensitive information into the container.
env:
  - name: LOG_LEVEL
    value: "debug"
# Secrets to mount when the container is executed.
# The 'name' field will be used to identify the secret in the system.
# If a secret is marked 'required', the system will ensure that the secret exists before allowing the container to run.
# Secrets can be mounted as environment variables or as files in a specific directory.
# The 'mountType' field specifies how the secret should be mounted (either `envVar` for environment variables or `file` for files).
# The 'mountTarget' field specifies the target location for the secret, a file path for `file` mountType or an environment variable name for `envVar` mountType.
secrets:
  - name: token
    mountType: envVar
    mountTarget: MY_TOKEN
```

### User Container With Parameters

```yaml
# A User Container With Parameters definition is the same as a User Container definition,
# with the addition of a `parameters` section. The other sections are the same as the User Container definition
# and are documented above.


# Container Image URI
image:  "myorg.domain/crawler:latest"
# Pull policy for the container image.
imagePullPolicy: IfNotPresent
# Command and args to execute when the container is run.
command: ["python3", "main.py"]
args:
  - "--verbose"
env:
  - name: LOG_LEVEL
    value: "debug"
secrets:
  - name: token
    mountType: envVar
    mountTarget: MY_TOKEN

# Parameter definitions for the container.
# These are provided when the container is invoked,
# For uploaders, it is expected to be defined in the profile that uses the uploader.
# For crawlers, it is expected to be defined when the crawler is invoked or scheduled to run.
parameters:
  # Parameter names should be in uppercase and use underscores.
  # This is due to the fact that the parameters are passed as environment variables to the container.
  # The parameter names will be converted to uppercase and underscores will be used instead of spaces.
  MY_REQUIRED_PARAMETER:
    description: The name of the GitHub organization to crawl.
    required: true # If true, The container will not be allowed to run without this parameter being provided.
  MY_OPTIONAL_PARAMETER:
    description: My description of the parameter.
    required: false # If false, The 'default' value will be used if the parameter is not provided.
    default: "default_value" 
```