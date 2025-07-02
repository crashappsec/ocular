# Default definitions

Ocular comes bundled with a set of default definitions that can be used to get started quickly.
See the sections below for more details on each type of definition.

## Index

- [Downloaders](#downloaders)
- [Uploaders](#uploaders)
- [Crawlers](#crawlers)
- ~~Profiles~~ *no default profiles are provided*
- ~~Secrets~~ *no default secrets are provided*

## Downloaders

Some of the default downloaders support authentication if certain secrets are set

<details>
<summary>git</summary>

Git downloader will interpret the target identifier as a git URL and will clone the repository to the local filesystem.
It will use the target version as the ref to check out, or the default branch if no version is specified.

#### Secrets

| Secret Name            | Required | Description                                                                                                                  |
|------------------------|----------|------------------------------------------------------------------------------------------------------------------------------|
| `downloader-gitconfig` | :x:      | The [gitconfig file](https://git-scm.com/docs/git-config#FILES) to use. Will be mounted to `/etc/gitconfig` in the container |

</details>

<details>
<summary>S3</summary>

S3 downloader will interpret the target identifier as a s3 bucket name and will clone the bucket to the local
filesystem.
It will use the target version as the object versions to pull down, or the latest version if no version is specified.

#### Secrets

| Secret Name            | Required | Description                                                                                                                                                   |
|------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `downloader-awsconfig` | :x:      | The [aws config file](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html) to use. Will be mounted to `/root/.aws/config` in the container. |

</details>

<details>
<summary>NPM</summary>

NPM downloader will interpret the target identifier as an npm package name, and will download the package `tar.gz` and
unpack it into the target directory.
It will use the target version as the package version to pull down, or the latest version if no version is specified.

#### Secrets

*Secrets are not currently supported for the npm downloader.*

</details>

<details>
<summary>PyPi</summary>

pypi downloader will interpret the target identifier as a PyPi package name, and will download all the packages files (
`.whl`, source files, etc.) to the target directory.
It will use the target version as the package version to pull down, or the latest version if no version is specified.

#### Secrets

*Secrets are not currently supported for the pypi downloader.*

</details>

<details>
<summary>GCS</summary>

GCS downloader will interpret the target identifier as a GCS bucket name and will clone the bucket to the local
filesystem.
It will use the target version as the object versions to pull down, or the latest version if no version is specified.

#### Secrets

| Secret Name                   | Required | Description                                                                                                                                                                                                          |
|-------------------------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gcs-application-credentials` | :x:      | The [google cloud credentials](https://cloud.google.com/docs/authentication/application-default-credentials#GAC) to use. Will be set as the environment variables `GOOGLE_APPLICATION_CREDENTIALS` in the container. |

</details>

<details>
<summary>docker</summary>

docker downloader will interpret the target identifier as a container URI will write the image locally, as `target.tar`
in the target directory.
It will use the target version as the image tag to pull down, or the `latest` if no version is specified.

#### Secrets

| Secret Name               | Required | Description                                                                                                                                                                            |
|---------------------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `downloader-dockerconfig` | :x:      | The [docker config](https://docs.docker.com/engine/swarm/configs/) file to use.Will be mounted to `/root/.docker/config.json`, and have that path set as the value for `DOCKER_CONFIG` |

</details>

## Uploaders

<details>
<summary>webhook</summary>

Webhook uploader will send the contents of the artifact as the body of an HTTP request.

#### Parameters

| Parameter | Description                            | Required           | Default |
|-----------|----------------------------------------|--------------------|---------|
| `URL`     | The URL to send the request to         | :white_check_mark: | -       |
| `METHOD`  | The HTTP method to use for the request | :white_check_mark: | -       |

#### Secrets

*Secrets are not currently supported for the webhook uploader.*

</details>

<details>
<summary>s3</summary>

S3 uploader will upload the contents of the artifact to an S3 bucket.

#### Parameters

| Parameter   | Description                                   | Required           | Default   |
|-------------|-----------------------------------------------|--------------------|-----------|
| `BUCKET`    | The name of the S3 bucket to upload to        | :white_check_mark: | -         |
| `SUBFOLDER` | The prefix to use for the uploaded object key | :x:                | ""        |
| `REGION`    | The AWS region of the S3 bucket               | :x:                | us-east-1 |

#### Secrets

*Secrets are not currently supported for the S3 uploader.*

</details>

## Crawlers

<details>
<summary>github</summary>

GitHub crawler will crawl GitHub organizations for all repositories and create pipelines for each repository found.

#### Parameters

| Parameter        | Description                                                      | Required           | Default                                                                                     |
|------------------|------------------------------------------------------------------|--------------------|---------------------------------------------------------------------------------------------|
| `PROFILE_NAME`   | The name of the profile to use for created pipelines             | :white_check_mark: |                                                                                             |
| `SLEEP_DURATION` | How long to sleep in between pipeline creations (e.g. 1m30s)     | :x:                | 2m                                                                                          |
| `DOWNLOADER`     | The name of the downloader to use for created pipelines          | :x:                | A default downloaders that best matches the service (i.e. github crawler -> git downloader) |
| `GITHUB_ORGS`    | The names of the github organizations to crawl (comma separated) | :white_check_mark: |                                                                                             |

#### Secrets

| Secret Name            | Required | Description                                                                                                                                                                                                                             |
|------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `crawler-github-token` | :x:      | The [github token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) to use for authentication. Will be set as the environment variable `GITHUB_TOKEN` in the container. |

</details>

<details>
<summary>gitlab</summary>

GitLab crawler will crawl GitLab groups for all repositories and create pipelines for each repository found.

#### Parameters

| Parameter        | Description                                                  | Required           | Default                                                                                     |
|------------------|--------------------------------------------------------------|--------------------|---------------------------------------------------------------------------------------------|
| `PROFILE_NAME`   | The name of the profile to use for created pipelines         | :white_check_mark: |                                                                                             |
| `SLEEP_DURATION` | How long to sleep in between pipeline creations (e.g. 1m30s) | :x:                | 2m                                                                                          |
| `DOWNLOADER`     | The name of the downloader to use for created pipelines      | :x:                | A default downloaders that best matches the service (i.e. github crawler -> git downloader) |
| `GITLAB_GROUPS`  | The names of the GitLab groups to crawl (comma separated)    | :white_check_mark: |                                                                                             |
| `GITLAB_URL`     | The URL of the GitLab instance to crawl                      | :x:                | https://gitlab.com                                                                          |

#### Secrets

| Secret Name            | Required | Description                                                                                                                                                                                 |
|------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `crawler-gitlab-token` | :x:      | The [gitlab token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) to use for authentication. Will be set as the environment variable `GITLAB_TOKEN` in the container. |

</details>

<details>
<summary>gitlab-instance </summary>

GitLab crawler will crawl an entire GitLab instance for all repositories and create pipelines for each repository found.

#### Parameters

| Parameter        | Description                                                  | Required           | Default                                                                                     |
|------------------|--------------------------------------------------------------|--------------------|---------------------------------------------------------------------------------------------|
| `PROFILE_NAME`   | The name of the profile to use for created pipelines         | :white_check_mark: |                                                                                             |
| `SLEEP_DURATION` | How long to sleep in between pipeline creations (e.g. 1m30s) | :x:                | 2m                                                                                          |
| `DOWNLOADER`     | The name of the downloader to use for created pipelines      | :x:                | A default downloaders that best matches the service (i.e. github crawler -> git downloader) |
| `GITLAB_URL`     | The URL of the GitLab instance to crawl                      | :x:                | https://gitlab.com                                                                          |

#### Secrets

| Secret Name            | Required | Description                                                                                                                                                                                 |
|------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `crawler-gitlab-token` | :x:      | The [gitlab token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) to use for authentication. Will be set as the environment variable `GITLAB_TOKEN` in the container. |

</details>


See the [definition section of the README](../../README.md#definitions) for more information on each of the definition types.