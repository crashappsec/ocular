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
YEAR ?= $(shell date +%Y)
OCULAR_CONTROLLER_REPOSITORY ?= ghcr.io/crashappsec/ocular-controller
export OCULAR_CONTROLLER_REPOSITORY
OCULAR_CONTROLLER_IMG ?= $(OCULAR_CONTROLLER_REPOSITORY):$(OCULAR_VERSION)
export OCULAR_CONTROLLER_IMG

OCULAR_SIDECAR_REPOSITORY ?= ghcr.io/crashappsec/ocular-sidecar
export OCULAR_SIDECAR_REPOSITORY
OCULAR_SIDECAR_IMG ?= $(OCULAR_SIDECAR_REPOSITORY):$(OCULAR_VERSION)
export OCULAR_SIDECAR_IMG

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
	"$(KUSTOMIZE)" build config/crd | "$(KUBECTL)" apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/crd | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

deploy-%: manifests kustomize ## Specify which config folder (%) to deploy to the K8s cluster specified in ~/.kube/config.
	"$(KUSTOMIZE)" build config/$* | "$(KUBECTL)" apply -f -

run-e2e-test-%:
	"$(KUSTOMIZE)" build config/e2e-test/$* | "$(KUBECTL)" apply -f -

stop-e2e-test-%:
	@"$(KUSTOMIZE)" build config/e2e-test/$* | \
		"$(YQ)" ea '[.] | reverse | .[]'  | \
		sed '/^apiVersion:/i ---' | \
		"$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -


.PHONY: refresh-deployment
refresh-deployment: ## Refresh the controller deployment in the K8s cluster specified in ~/.kube/config.
	"$(KUBECTL)" rollout restart deployment/ocular-controller-manager -n ocular-system

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

undeploy-%: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/$* | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -


##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: license-eye controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."
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

.PHONY: update-github-actions
update-github-actions: frizbee ## Update GitHub action versions in workflows
	@echo "Updating GitHub workflows ..."
	@"$(FRIZBEEE)" actions .github/workflows


##@ Testing

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# kubectl kuberc is disabled by default for test isolation; enable with:
# - KUBECTL_KUBERC=true
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
			echo "Creating Kind cluster '$(KIND_CLUSTER)'... $(DOCKER_DEFAULT_PLATFORM)"; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

##@ Linting

.PHONY: lint
lint: golangci-lint license-eye ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run
	"$(LICENSE_EYE)" header check

.PHONY: lint-fix
lint-fix: golangci-lint license-eye ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix
	"$(LICENSE_EYE)" header fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

.PHONY: license-fix
license-fix: ## Fix license headers
	@echo "Formatting license headers ..."
	@"$(LICENSE_EYE)" header fix

GHASOURCEDIR := ./.github/workflows
GHASOURCES := $(shell find $(GHASOURCEDIR) -name '*.yaml')
.PHONY: gha-upgrade
gha-upgrade: ratchet ## upgrades all pinned github actions used in any workflows
	@"$(RATCHET)" upgrade $(GHASOURCES)

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build controller binary.
	go build -o bin/controller cmd/controller/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/controller/main.go

.PHONY: docker-build-all
docker-build-all: docker-build-controller docker-build-sidecar ## Builds all docker images

# PLATFORMS is a list of platforms to
# build for. Production Ocular images are built
# with: 'linux/arm64,linux/amd64,linux/s390x,linux/ppc64le'
PLATFORMS ?= linux/arm64,linux/amd64

# Additionally, docker args can be set,
# adding --push will push the image
DOCKER_ARGS ?= --platform=$(PLATFORMS)

LDFLAGS ?= -X main.version=$(OCULAR_VERSION) -X main.buildTime=$(shell date -Iseconds) -X main.gitCommit=$(shell git rev-parse --short HEAD)
.PHONY: docker-build-controller
docker-build-controller:  docker-build-img-controller ## Build docker image for the manager.

.PHONY: docker-build-sidecar
docker-build-sidecar: docker-build-img-sidecar ## Build docker image for the sidecar.

docker-build-img-%: ## Builds the docker image
	$(CONTAINER_TOOL) build \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		--build-arg COMMAND=$* \
		--tag $(OCULAR_$(shell echo '$*' | tr '[:lower:]' '[:upper:]')_IMG) \
		$(DOCKER_ARGS) \
		-f Dockerfile .

.PHONY: docker-push-all
docker-push-all: docker-push-controller docker-push-sidecar ## Push docker both manager and sidecar images.

.PHONY: docker-push-controller
docker-push-controller: docker-push-img-controller ## Push docker image with the manager.

.PHONY: docker-push-sidecar
docker-push-sidecar: docker-push-img-sidecar ## Push docker image with the sidecar.

docker-push-img-%: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push $(OCULAR_$(shell echo '$*' | tr '[:lower:]' '[:upper:]')_IMG)
.PHONY: build-installer
build-installer: manifests generate kustomize yq ## Generate a consolidated YAML with CRDs and deployment.
	@mkdir -p dist
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

build-installer-%: manifests generate kustomize yq ## Generate a consolidated YAML with CRDs and deployment from a specific config folder.
	@mkdir -p dist
	"$(KUSTOMIZE)" build config/$(@:build-installer-%=%) > dist/install-$(@:build-installer-%=%).yaml

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

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
KUBEBUILDER ?= $(LOCALBIN)/kubebuilder
RATCHET ?= $(LOCALBIN)/ratchet
# https://book.kubebuilder.io/plugins/extending/external-plugins.html#how-to-use-an-external-plugin
HELMPATCH_NAME=helm.ocular.crashoverride.run
HELMPATCH_VERSION=v1-alpha
HELMPATCH_DIR ?= $(LOCALBIN)/$(HELMPATCH_NAME)/$(HELMPATCH_VERSION)
HELMPATCH_PLUGIN ?= $(HELMPATCH_DIR)/$(HELMPATCH_NAME)
HELMPATCH_SOURCES ?= $(wildcard hack/kubebuilder-helm-patch/*.go)


## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.21.0
GOLANGCI_LINT_VERSION ?= v2.12.2
YQ_VERSION ?= v4.53.2
CODE_GENERATOR_VERSION ?= v0.36.0
LICENSE_EYE_VERSION ?= v0.8.0
KUBEBUILDER_VERSION ?= v4.13.1
RATCHET_VERSION ?= v0.11.4

#ENVTEST_VERSION is the controller-runtime version to use for setup-envtest, derived from go.mod
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v")

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

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
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
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
	@test -f .custom-gcl.yml && { \
		echo "Building custom golangci-lint with plugins..." && \
		$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom && \
		mv -f $(LOCALBIN)/golangci-lint-custom $(GOLANGCI_LINT); \
	} || true

yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,$(YQ_VERSION))

client-gen: $(CLIENT_GEN) ## Download code-generator locally if necessary.
$(CLIENT_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen,$(CODE_GENERATOR_VERSION))

license-eye: $(LICENSE_EYE) ## Download skywalking-eyes locally if necessary.
$(LICENSE_EYE): $(LOCALBIN)
	$(call go-install-tool,$(LICENSE_EYE),github.com/apache/skywalking-eyes/cmd/license-eye,$(LICENSE_EYE_VERSION))

ratchet: $(RATCHET) ## Download ratchet locally if necessary.
$(RATCHET): $(LOCALBIN)
	$(call go-install-tool,$(RATCHET),github.com/sethvargo/ratchet,$(RATCHET_VERSION))

kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	$(call go-install-tool,$(KUBEBUILDER),sigs.k8s.io/kubebuilder/v4,$(KUBEBUILDER_VERSION))

.PHONY: helmpatch-plugin
helmpatch-plugin: $(HELMPATCH_PLUGIN)
$(HELMPATCH_PLUGIN): $(LOCALBIN) $(HELMPATCH_SOURCES)
	@mkdir -p $(HELMPATCH_DIR)
	@echo "Compiling kubebuilder helm patch plugin"
	@cd hack/kubebuilder-helm-patch && go build -o $(HELMPATCH_PLUGIN)



# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

##@ Helm Deployment

## Helm binary to use for deploying the chart
HELM ?= helm
## Namespace to deploy the Helm release
HELM_NAMESPACE ?= ocular-system
## Name of the Helm release
HELM_RELEASE ?= ocular
HELM_OUTPUT_DIR ?= dist
## Path to the Helm chart directory
HELM_CHART_DIR ?= $(HELM_OUTPUT_DIR)/chart
## Additional arguments to pass to helm commands
HELM_EXTRA_ARGS ?=


OCULAR_HELM_VERSION ?= 0.0.0-dev
export OCULAR_HELM_VERSION
# Since Ocular has additional chart variables that are not covered by
# 'helm.kubebuilder.io/v2-alpha', A custom kubebuilder plugin is used to template
# files after they are generated by the helm plugin. We then set some default values
# using YQ since it preseves comments, unlike Go's YAML parser. 
.PHONY: helm-build
helm-build: kubebuilder helmpatch-plugin yq ## Generate a helm-chart using kubebuilder
	@mkdir -p dist
	@EXTERNAL_PLUGINS_PATH="$(LOCALBIN)" "$(KUBEBUILDER)" edit --plugins=helm.kubebuilder.io/v2-alpha,$(HELMPATCH_NAME)/$(HELMPATCH_VERSION) --output-dir=$(HELM_OUTPUT_DIR)
	@"$(YQ)" -ie '.manager.labels = {}' $(HELM_CHART_DIR)/values.yaml
	@"$(YQ)" -ie '.manager.annotations = {}' $(HELM_CHART_DIR)/values.yaml
	@"$(YQ)" -ie '.manager.image = {"repository": strenv(OCULAR_CONTROLLER_REPOSITORY), "pullPolicy": "IfNotPresent", "tag": "v{{ .Chart.AppVersion }}"}' $(HELM_CHART_DIR)/values.yaml
	@"$(YQ)" -ie '.sidecar.image =  {"repository": strenv(OCULAR_SIDECAR_REPOSITORY), "pullPolicy": "IfNotPresent", "tag": "v{{ .Chart.AppVersion }}"}' $(HELM_CHART_DIR)/values.yaml
	@"$(YQ)" -ie '(.sidecar | key) head_comment="Configure Ocular sidecar image"' $(HELM_CHART_DIR)/values.yaml
	@"$(YQ)" -ie '.appVersion = (strenv(OCULAR_VERSION) | sub("^v", ""))' $(HELM_CHART_DIR)/Chart.yaml



.PHONY: helm-clean
clean-helm: ## Clean up the helm chart generated files
	@rm -rf $(HELM_CHART_DIR)/Chart.yaml $(HELM_CHART_DIR)/templates/ $(HELM_CHART_DIR)/values.yaml


.PHONY: install-helm
install-helm: ## Install the latest version of Helm.
	@command -v $(HELM) >/dev/null 2>&1 || { \
		echo "Installing Helm..." && \
		curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | bash; \
	}

.PHONY: helm-deploy
helm-deploy: install-helm ## Deploy manager to the K8s cluster via Helm. Specify an image with IMG.
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set manager.image.repository=$${OCULAR_CONTROLLER_IMG%:*} \
		--set manager.image.tag=$${OCULAR_CONTROLLER_IMG##*:} \
		--set sidecar.image.repository=$${OCULAR_SIDECAR_IMG%:*} \
		--set sidecar.image.tag=$${OCULAR_SIDECAR_IMG##*:} \
		--wait \
		--timeout 5m \
		$(HELM_EXTRA_ARGS)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the Helm release from the K8s cluster.
	$(HELM) uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-status
helm-status: ## Show Helm release status.
	$(HELM) status $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-history
helm-history: ## Show Helm release history.
	$(HELM) history $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-rollback
helm-rollback: ## Rollback to previous Helm release.
	$(HELM) rollback $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)
