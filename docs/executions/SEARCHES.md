# Searches

Searches are used to find targets that can be scanned by the pipeline.
They are typically used to discover new targets or to find targets that match certain criteria.
Searches are executed by running a crawler, which is a container image that is expected to gather a set of targets to scan and call the API to start scans for those targets.
The container will be given an authenticated token to the API, allowing it to call the API to start scans or other crawlers.
The token is located in a file mounted at the path specified by the environment variable `OCULAR_SERVICE_ACCOUNT_TOKEN_PATH`.
The API base URL is also provided as an environment variable (`OCULAR_API_BASE_URL`).

The search will execute the following steps:
1. **Run Crawler**: The search will run the specified crawler with the parameters provided in the request.
   The crawler is expected to gather a set of targets to scan and call the API to start scans for those targets.
2. **Start Pipelines**: Once the crawler has gathered the targets, it should call the API to start pipelines for each target.
   The pipelines will execute as normal (see [Pipelines](/docs/executions/PIPELINES.md) for more details).
   NOTE: crawlers should space out the pipeline creation to avoid overwhelming the system with too many pipelines at once.
   (A solution to this is actively being worked on, but currently the crawler should implement its own throttling logic.)
3. **Complete**: Once the crawler has completed, the search will be considered complete.

Currently, there is no feedback mechanism for the search execution, so the user will need to check the API status of the search execution to see if it was successful or not.

## Definitions

### Request

A search request is a simple YAML or JSON object that contains the following fields:

```yaml
# Example search request
# It can be sent to the endpoint `POST /api/v1/searches`
# with the body containing the request in YAML or JSON format (depending on Content-Type header).

crawlerName: "my-custom-crawler" # Name of the crawler to run
parameters: # Parameters to pass to the crawler
  GITHUB_ORGS: "myorg" # Example parameter, the crawler should define its own parameters
  SLEEP_DURATION: "1m30s" # Example parameter, the crawler should define its own parameters
```

If the search is scheduled, the request should use the endpoint `POST /api/v1/scheduled/searches` with the addition
of the `schedule` field, which is a cron expression that defines when the search should be executed.

```yaml
# Example scheduled search request
# It can be sent to the endpoint `POST /api/v1/scheduled/searches`
# with the body containing the request in YAML or JSON format (depending on Content-Type header).

crawlerName: "my-nightly-crawler" # Name of the crawler to run
parameters: # Parameters to pass to the crawler
  GITHUB_ORGS: "myorg" # Example parameter, the crawler should define its own parameters
  SLEEP_DURATION: "1m30s" # Example parameter, the crawler should define its own parameters
schedule: "0 0 * * *" # Cron expression for the schedule (e.g. every day at midnight)
```

### Response

The response to a pipeline request is the state of the pipeline execution. This can be queried via the endpoint `GET /api/v1/schedules/{id}`.
or `GET /api/v1/scheduled/searches/{id}` for scheduled searches.

```yaml
id: "12345678-1234-1234-1234-123456789012" # Unique identifier of the pipeline execution
state: "Pending" # Current state of the pipeline execution (e.g. Pending, Running, Completed, Failed)
# The state will start as "Pending" and will change to "Running" when the downloader container begins its execution.
crawler: "my-custom-profile" # Name of the profile used for this pipeline
parameters: # Parameters passed to the crawler
  GITHUB_ORGS: "myorg" # Example parameter, the crawler should define its own parameters
  SLEEP_DURATION: "1m30s" # Example parameter, the crawler should define its own parameters
# schedule: "0 0 * * *" # Cron expression for the schedule (if was scheudled search)
```