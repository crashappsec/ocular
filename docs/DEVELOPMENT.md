# Development

This document describes the development process for the project.

## Getting Started

*NOTE*: This project is built using [Kubebuilder](https://kubebuilder.io/).
The following documenation is taken and adapted from the default README generated
by Kubebuilder. For more information, please refer to the [Kubebuilder documentation](https://kubebuilder.io/).

For a more in-depth guide on how to get started with the project and what the different components
are, please refer to the [documentation site](https://ocularproject.io/docs/)

### Prerequisites
- go
- docker
- kubectl
- Access to a Kubernetes v1.28.0+ cluster.

*NOTE*: Any environment variable mentioned in the following commands can be set in the
`.env` file (or whatever file you set `OCULAR_ENV_FILE` to), which is loaded automatically by the `make` command.
An example `.env` file is provided in the repository as [`example.env`](/example.env).

There are two images required inorder to run 'ocular':
- `OCULAR_CONTROLLER_IMG`: The image for the controller manager. This is a webserver that
  will act as a kubernetes controller and will manage the lifecycle of all Ocular resources.
- `OCULAR_EXTRACTOR_IMG`: The image for the extractor. This is a program that facilities
  the extraction of artifacts from the scanners to uploaders in a pipeline.

### To Deploy on the cluster

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image**

```sh
make deploy
```


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

### To Deploy with custom images

**Build and push your image to the location specified by `OCULAR_CONTROLLER_IMG` and `OCULAR_EXTRACTOR_IMG`:**

```sh
# Controller image
make docker-build-controller docker-push-controller \
  OCULAR_CONTROLLER_IMG=<some-registry>/ocular-controller:tag

# Extractor image
make docker-build-extractor docker-push-extractor \
  OCULAR_EXTRACTOR_IMG=<some-registry>/ocular-extractor:tag
  
# Or both at once
make docker-build-all docker-push-all \
  OCULAR_CONTROLLER_IMG=<some-registry>/ocular-controller:tag \
  OCULAR_EXTRACTOR_IMG=<some-registry>/ocular-extractor:tag

```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Create a custom deployment configuration (in this example we will name it `dev`)**

```sh
export DEPLOYMENT_NAME=dev \
       OCULAR_CONTROLLER_IMG=<some-registry>/ocular-controller:tag \
       OCULAR_EXTRACTOR_IMG=<some-registry>/ocular-extractor:tag
# NOTE you should make this folder will be ignored by git
# you can check the .gitignore file for which config folders are ignored
# and use one of those names to avoid committing custom configs by mistake
mkdir -p config/${DEPLOYMENT_NAME}
cd config/${DEPLOYMENT_NAME}

kustomize create
kustomize edit add resource ../default
kustomize edit set image ghcr.io/crashappsec/ocular-controller=${OCULAR_CONTROLLER_IMG}
kustomize edit set configmap  controller-manager-config --from-literal=OCULAR_EXTRACTOR_IMG=${OCULAR_EXTRACTOR_IMG}
```


**Deploy the custom configuration to the cluster with the folder specified by ${DEPLOYMENT_NAME}:**

```sh
make deploy-${DEPLOYMENT_NAME}
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

### To Uninstall a custom deployment

**UnDeploy the controller from the cluster:**

```sh
make undeploy-${DEPLOYMENT_NAME}
```

>**NOTE**: Ensure that the samples has default values to test it out.


## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer \
  OCULAR_CONTROLLER_IMG=<some-registry>/ocular-controller:tag \
  OCULAR_EXTRACTOR_IMG=<some-registry>/ocular-extractor:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f ./dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

**NOTE**: This should only be used if you know what you are doing. Users should prefer installation from the [helm charts repository](https://github.com/crashappsec/helm-charts).

```sh
make build-helm \
  OCULAR_CONTROLLER_IMG=<some-registry>/ocular-controller:tag \
  OCULAR_EXTRACTOR_IMG=<some-registry>/ocular-extractor:tag
```

2. See that a chart was generated under 'dist/chart', and users
   can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.



