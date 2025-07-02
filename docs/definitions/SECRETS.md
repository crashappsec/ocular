# Secrets

Secrets container sensitive information that is required by containers to run.
Secrets can be mounted as environment variables or as files in a specific directory.
A secret associates data with a name in the API.

Secrets are referenced by their name in any user container or user container with parameters definition.
See [CONTAINERS.md](/docs/definitions/CONTAINERS.md) for more details on how to define a user container or user container with parameters.

## Definition

```yaml
# Example definition of a secret.
# It can be set by calling the endpoint `POST /api/v1/secrets/{name}`
# It will take the raw data from the request body and store it in the system.
my-secret-value
```
