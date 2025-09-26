# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

OCULAR_ENV_FILE ?= .env

# Only if .env file is present
ifneq (,$(wildcard ${OCULAR_ENV_FILE}))
	include ${OCULAR_ENV_FILE}
endif


OCULAR_VERSION ?= $(shell git describe --tags --dirty=-dev)
export OCULAR_VERSION
# Image URL to use all building/pushing image targets
OCULAR_CONTROLLER_IMG ?= ghcr.io/crashappsec/ocular-controller:$(OCULAR_VERSION)
export OCULAR_CONTROLLER_IMG
OCULAR_EXTRACTOR_IMAGE ?= $(OCULAR_CONTROLLER_IMG)
export OCULAR_EXTRACTOR_IMAGE


# This is the default image for the ocular controller. Updating the image
# via kustomize writes to disk, so we store this value to revert after any
# build/deploy commands are used. This is used in the 'revert-image' target.
DEFAULT_OCULAR_CONTROLLER_IMG ?= ghcr.io/crashappsec/ocular-controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec



.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && yq -ie '.[0].value.value = strenv(OCULAR_EXTRACTOR_IMAGE)' extractor-patch.yaml && $(KUSTOMIZE) edit set image controller=${OCULAR_CONTROLLER_IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -
	$(MAKE) revert-image

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(MAKE) generate-clientset

.PHONY: generate-clientset
generate-clientset: client-gen ## Generate clientset for our CRDs.
	$(CLIENT_GEN) \
 		--input-base "" \
		--input "github.com/crashappsec/ocular/api/v1beta1" \
	 	--clientset-name "clientset" \
		--output-dir "pkg/generated" \
		--output-pkg "github.com/crashappsec/ocular/pkg/generated" \
		--go-header-file "hack/boilerplate.go.txt" \

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= ocular-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND_CLUSTER=$(KIND_CLUSTER) go test ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint license-eye ## Run golangci-lint linter
	$(GOLANGCI_LINT) run
	$(LICENSE_EYE) header check

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix
	$(LICENSE_EYE) header fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

CURRENT_DIR := $(shell pwd)

license-fix: ## Fix license headers
	@echo "Formatting license headers ..."
	@$(LICENSE_EYE) header fix

update-github-actions: frizbee ## Update GitHub action versions in workflows
	@echo "Updating GitHub workflows ..."
	@$(FRIZBEEE) actions .github/workflows


##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${OCULAR_CONTROLLER_IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${OCULAR_CONTROLLER_IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx OCULAR_CONTROLLER_IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via OCULAR_CONTROLLER_IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	@echo -e "This will build and \e[31m$$(tput bold)push$$(tput sgr0)\e[0m the image ${OCULAR_CONTROLLER_IMG} for platforms: ${PLATFORMS}."
	@read -p "press enter to continue, or ctrl-c to abort: "
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name ocular-builder
	$(CONTAINER_TOOL) buildx use ocular-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${OCULAR_CONTROLLER_IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm ocular-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize yq ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && yq -ie '.[0].value.value = strenv(OCULAR_EXTRACTOR_IMAGE)' extractor-patch.yaml && $(KUSTOMIZE) edit set image controller=${OCULAR_CONTROLLER_IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml
	@$(MAKE) revert-image

.PHONY: build-helm
build-helm: manifests generate kustomize yq ## Generate a helm-chart using kubebuilder
	@mkdir -p dist
	@# hack to get the env var to be templated in the chart, since currrently helm/v2-alpha doesn't support it
	@# first we set the image to a replaceme value, then we replace it in the generated chart
	@OCULAR_EXTRACTOR_IMAGE=extractor-image-replaceme $(KUBEBUILDER) edit --plugins=helm/v2-alpha
	@yq -ie '.controllerManager.image.tag = strenv(OCULAR_VERSION)' dist/chart/values.yaml
	@yq -ie '.appVersion = (strenv(OCULAR_VERSION) | sub("^v", ""))' dist/chart/Chart.yaml
	@# replace the image with the helm template values
	@sed -i.bak 's|extractor-image-replaceme|{{ .Values.controllerManager.image.repository }}:{{ .Values.controllerManager.image.tag }}|g' dist/chart/templates/manager/manager.yaml && rm dist/chart/templates/manager/manager.yaml.bak
	$(MAKE) revert-image

.PHONY: clean-helm
clean-helm: ## Clean up the helm chart generated files
	@rm -rf dist/chart

.PHONY: revert-image
revert-image: kustomize ## Revert the image in the kustomization to the default image.
	@cd config/manager && yq -ie '.[0].value.value = strenv(DEFAULT_OCULAR_CONTROLLER_IMG)' extractor-patch.yaml && $(KUSTOMIZE) edit set image controller=${DEFAULT_OCULAR_CONTROLLER_IMG}

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
YQ ?= $(LOCALBIN)/yq
CLIENT_GEN ?= $(LOCALBIN)/client-gen
LICENSE_EYE ?= $(LOCALBIN)/license-eye
FRIZBEEE ?= $(LOCALBIN)/frizbee
KUBEBUILDER ?= $(LOCALBIN)/kubebuilder

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v2.5.0
YQ_VERSION ?= v4.47.1
CODE_GENERATOR_VERSION ?= v0.34.0
LICENSE_EYE_VERSION ?= v0.7.0
FRIZBEEE_VERSION ?=  0.1.7
KUBEBUILDER_VERSION ?= master # support for helm-v2 isn't available in a release yet

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,$(YQ_VERSION))

client-gen: $(CLIENT_GEN) ## Download code-generator locally if necessary.
$(CLIENT_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen,$(CODE_GENERATOR_VERSION))

license-eye: $(LICENSE_EYE) ## Download skywalking-eyes locally if necessary.
$(LICENSE_EYE): $(LOCALBIN)
	$(call go-install-tool,$(LICENSE_EYE),github.com/apache/skywalking-eyes/cmd/license-eye,$(LICENSE_EYE_VERSION))

license-eye: $(FRIZBEEE) ## Download skywalking-eyes locally if necessary.
$(FRIZBEEE): $(LOCALBIN)
	$(call go-install-tool,$(FRIZBEEE),github.com/stacklok/frizbee,$(FRIZBEEE_VERSION))

kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	$(call go-install-tool,$(KUBEBUILDER),sigs.k8s.io/kubebuilder/v4,$(KUBEBUILDER_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
