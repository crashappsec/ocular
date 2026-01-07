# Copyright (C) 2025-2026 Crash Override, Inc.
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


OCULAR_VERSION ?= latest
export OCULAR_VERSION
# Image URL to use all building/pushing image targets
OCULAR_CONTROLLER_IMG ?= ghcr.io/crashappsec/ocular-controller:$(OCULAR_VERSION)
export OCULAR_CONTROLLER_IMG
OCULAR_EXTRACTOR_IMG ?= ghcr.io/crashappsec/ocular-extractor:$(OCULAR_VERSION)
export OCULAR_EXTRACTOR_IMG


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

LDFLAGS ?= -X main.version=$(OCULAR_VERSION) -X main.buildTime=$(shell date -Iseconds) -X main.gitCommit=$(shell git rev-parse --short HEAD)

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
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

deploy-%: manifests kustomize ## Specify which config folder (%) to deploy to the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/$(@:deploy-%=%) | $(KUBECTL) apply -f -

.PHONY: refresh-deployment
refresh-deployment: ## Refresh the controller deployment in the K8s cluster specified in ~/.kube/config.
	$(KUBECTL) rollout restart deployment/ocular-controller-manager -n ocular-system

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

undeploy-%: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/$(@:deploy-%=%) | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -


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
	@$(CLIENT_GEN) \
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
COMMANDS := controller extractor

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build-all
docker-build-all: docker-build-controller docker-build-extractor ## Build docker image with the manager.

.PHONY: docker-build-controller
docker-build-controller:  docker-build-img-controller ## Build docker image with the manager.

.PHONY: docker-build-extractor
docker-build-extractor: docker-build-img-extractor ## Build docker image with the extractor.

docker-build-img-%:
	$(CONTAINER_TOOL) build --build-arg LDFLAGS="$(LDFLAGS)" --build-arg COMMAND=$(@:docker-build-img-%=%) -t $(OCULAR_$(shell echo '$(@:docker-build-img-%=%)' | tr '[:lower:]' '[:upper:]')_IMG) .

.PHONY: docker-push-all
docker-push-all: docker-push-controller docker-push-extractor ## Push docker both manager and extractor images.

.PHONY: docker-push-controller
docker-push-controller: docker-push-img-controller ## Push docker image with the manager.

.PHONY: docker-push-extractor
docker-push-extractor: docker-push-img-extractor ## Push docker image with the extractor.

docker-push-img-%: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push $(OCULAR_$(shell echo '$(@:docker-build-%=%)' | tr '[:lower:]' '[:upper:]')_IMG)

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx OCULAR_CONTROLLER_IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via OCULAR_CONTROLLER_IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx-all
docker-buildx-all: docker-buildx-controller  docker-buildx-extractor ## Build and push docker images for both manager and extractor for cross-platform support.

.PHONY: docker-buildx-controller
docker-buildx-controller: docker-buildx-img-controller ## Build and push docker image for the manager for cross-platform support

.PHONY: docker-buildx-extractor
docker-buildx-extractor: docker-buildx-img-extractor ## Build and push docker image for the extractor for cross-platform support

docker-buildx-img-%: ## Build and push docker image for the manager for cross-platform support
	@echo -e "This will build and \e[31m$$(tput bold)push$$(tput sgr0)\e[0m the image $(OCULAR_$(shell echo '$(@:docker-buildx-img-%=%)' | tr '[:lower:]' '[:upper:]')_IMG) for platforms: ${PLATFORMS}."
	@read -p "press enter to continue, or ctrl-c to abort: "
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name ocular-builder
	$(CONTAINER_TOOL) buildx use ocular-builder
	- $(CONTAINER_TOOL) buildx build --build-arg LDFLAGS="$(LDFLAGS)" --build-arg COMMAND=$(@:docker-buildx-img-%=%) --push --platform=$(PLATFORMS) --tag $(OCULAR_$(shell echo '$(@:docker-buildx-img-%=%)' | tr '[:lower:]' '[:upper:]')_IMG) -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm ocular-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize yq ## Generate a consolidated YAML with CRDs and deployment.
	@mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml

build-installer-%: manifests generate kustomize yq ## Generate a consolidated YAML with CRDs and deployment from a specific config folder.
	@mkdir -p dist
	$(KUSTOMIZE) build config/$(@:build-installer-%=%) > dist/install-$(@:build-installer-%=%).yaml

# The build-helm chart is a bit hacky right now since kubebuilder doesn't support
# adding really any customizations to the 'helm/v2-alpha' plugin. So we do some
# sed/yq magic to get the values we want into the chart after we generate it from
# the 'config' kustomization.
# Steps to build helm are:
# 1. Ensure the helm chart is copied over from the repo 'crashappsec/helm-charts' to 'dist/chart'
#    The helm chart is stored there to allow for versioning and easier updates.
# 2. We use 'kubebuilder edit --plugins=helm/v2-alpha' to update the helm chart as a result
#    of the 'config' kustomization. This will overwrite the existing resources in 'dist/chart'
# 3. We then use yq/sed to update the values.yaml and Chart.yaml with any customizations we want
#    to ensure they weren't lost during the 'kubebuilder edit' step.
# 4. [TO DISTRIBUTE] we then copy the chart to the 'crashappsec/helm-charts' repo for distribution.
# NOTE: this target only performs steps 2 and 3. Step 1 is a manual step that must be
#       performed by the developer to ensure the chart is up-to-date before running
#       this target. Step 4 is also a manual step to push the updated chart to
#       the 'crashappsec/helm-charts' repo for distribution.
.PHONY: build-helm
build-helm: kubebuilder ## Generate a helm-chart using kubebuilder
	@mkdir -p dist
	@$(KUBEBUILDER) edit --plugins=helm/v2-alpha
	@# update manfiests with any templating or customizations TODO(bryce): have this be one script
	@sed -i.bak -r 's/^([ ]+)labels:/\1labels:\n\1    {{- range $$key, $$val := .Values.manager.labels }}\n    \1{{ $$key }}: {{ $$val | quote }}\n\1    {{- end}}/g' dist/chart/templates/manager/manager.yaml
	@sed -i.bak -r 's/^([ ]+)annotations:/\1annotations:\n\1    {{- range $$key, $$val := .Values.manager.annotations }}\n    \1{{ $$key }}: {{ $$val | quote }}\n\1    {{- end}}/g' dist/chart/templates/manager/manager.yaml
	@sed -i.bak -r 's/^([ ]+)volumeMounts:/\1volumeMounts:\n\1  {{- with .Values.manager.volumeMounts }}\n\1  {{- toYaml . | nindent 20}}\n\1  {{- end}}/g' dist/chart/templates/manager/manager.yaml
	@sed -i.bak -r 's/^([ ]+)volumes:/\1volumes:\n\1    {{- with .Values.manager.volumes }}\n\1    {{- toYaml . | nindent 16}}\n\1    {{- end}}/g' dist/chart/templates/manager/manager.yaml
	@sed -i.bak -r 's/^([ ]+OCULAR_EXTRACTOR_IMG:)[^\n]+/\1 "{{ .Values.extractor.image.repository }}:{{ .Values.extractor.image.tag }}"/g' dist/chart/templates/other/other.yaml
	@sed -i.bak -r 's/^([ ]+value:[ ]+)["]?IfNotPresent["]?$$/\1 "{{ .Values.extractor.image.pullPolicy }}"/g' dist/chart/templates/manager/manager.yaml
	@rm dist/chart/templates/manager/manager.yaml.bak dist/chart/templates/other/other.yaml.bak  # cleanup backup file from sed
	@yq -ie '.manager.image.tag = strenv(OCULAR_VERSION)' dist/chart/values.yaml
	@yq -ie '.extractor.image.tag = strenv(OCULAR_VERSION)' dist/chart/values.yaml
	@yq -ie '.appVersion = (strenv(OCULAR_VERSION) | sub("^v", ""))' dist/chart/Chart.yaml

.PHONY: clean-helm
clean-helm: ## Clean up the helm chart generated files
	@rm -rf dist/chart

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
FRIZBEEE_VERSION ?=  v0.1.7
KUBEBUILDER_VERSION ?= v4.10.1

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

frizbee: $(FRIZBEEE) ## Download frizbee locally if necessary.
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
