# Development

This document describes the development process for the project.

## Getting Started

*NOTE*: This project is built using [Kubebuilder](https://kubebuilder.io/).
The following documenation is taken and adapted from the default README generated
by Kubebuilder. For more information, please refer to the [Kubebuilder documentation](https://kubebuilder.io/).

For a more in-depth guide on how to get started with the project and what the different components
are, please refer to the [documentation site](https://ocularproject.io/docs/)

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

*NOTE*: Any environment variable mentioned in the following commands can be set in the
`.env` file (or whatever file you set `OCULAR_ENV_FILE` to), which is loaded automatically by the `make` command.
An example `.env` file is provided in the repository as [`example.env`](/example.env).

### To Deploy on the cluster
**Build and push your image to the location specified by `OCULAR_CONTROLLER_IMG`:**

```sh
make docker-build docker-push OCULAR_CONTROLLER_IMG=<some-registry>/ocular:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `OCULAR_CONTROLLER_IMG`:**

```sh
make deploy OCULAR_CONTROLLER_IMG=<some-registry>/ocular:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer OCULAR_CONTROLLER_IMG=<some-registry>/ocular:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/crashappsec/ocular/main/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
   can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.



