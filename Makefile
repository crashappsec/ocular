# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

GO_SOURCES=$(shell find . -name '*.go' -not -path './cmd/*' -not -path './hack/*')
STATIC_SOURCE=$(shell find . -type f -path './pkg/api/static/*' -not -path './pkg/api/static/embed.go')
API_SERVER_SOURCES=$(shell find cmd/api-server -name '*.go')
EXTRACTOR_SOURCES=$(shell find cmd/extractor -name '*.go')

DEP_SOURCES := go.sum go.mod

OCULAR_ENV_FILE ?= .env

OCULAR_VERSION ?= $(shell git describe --exact-match --tags 2>/dev/null || echo "dev")
OCULAR_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
OCULAR_BUILD_TIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

CURRENT_DIR := $(shell pwd)

# Only if .env file is present
ifneq (,$(wildcard ${OCULAR_ENV_FILE}))
	include ${OCULAR_ENV_FILE}
endif

OCULAR_ENVIRONMENT ?= development
BASE_LD_FLAGS := -X github.com/crashappsec/ocular/internal/config.Version=${OCULAR_VERSION} -X github.com/crashappsec/ocular/internal/config.Commit=${OCULAR_COMMIT} -X github.com/crashappsec/ocular/internal/config.BuildTime=${OCULAR_BUILD_TIME}
ifeq ($(OCULAR_ENVIRONMENT), production)
	LDFLAGS:= -w -s ${BASE_LD_FLAGS}
else
	LDFLAGS:= ${BASE_LD_FLAGS}
endif

# logging level debug when using make
OCULAR_LOGGING_LEVEL ?= debug

OCULAR_PROFILE_CONFIGMAPNAME ?= ocular-profiles
OCULAR_CRAWLERS_CONFIGMAPNAME ?= ocular-crawlers
OCULAR_UPLOADERS_CONFIGMAPNAME ?= ocular-uploaders
OCULAR_DOWNLOADERS_CONFIGMAPNAME ?= ocular-donwloaders
OCULAR_SECRETS_SECRETNAME ?= ocular-secrets
OCULAR_SERVICE_ACCOUNT ?= ocular-admin

DOCKER_BUILDKIT ?= 1

ifneq ($(DOCKER_DEFAULT_PLATFORM),)
	export DOCKER_DEFAULT_PLATFORM
endif
OCULAR_IMAGE_REGISTRY ?= ghcr.io
OCULAR_IMAGE_TAG ?= local
OCULAR_API_SERVER_IMAGE_REPOSITORY ?= crashappsec/ocular-api-server
OCULAR_EXTRACTOR_IMAGE_REPOSITORY ?= crashappsec/ocular-extractor

OCULAR_API_SERVER_IMAGE ?= ${OCULAR_IMAGE_REGISTRY}/${OCULAR_API_SERVER_IMAGE_REPOSITORY}:${OCULAR_IMAGE_TAG}
OCULAR_EXTRACTOR_IMAGE ?= ${OCULAR_IMAGE_REGISTRY}/${OCULAR_EXTRACTOR_IMAGE_REPOSITORY}:${OCULAR_IMAGE_TAG}

export

.PHONY: all clean
all: build-docker

clean:
	@echo "Cleaning up build artifacts ..."
	@rm -rf bin
	@rm -f coverage.out

#########
# Build #
#########

.PHONY: build build-api-server build-downloader build-crawler
build: build-api-server build-extractor

build-api-server: bin/api-server
build-extractor: bin/extractor

bin/extractor: cmd/extractor/main.go  $(EXTRACTOR_SOURCES) $(GO_SOURCES) $(DEP_SOURCES)
	@go build -o $@ -ldflags='${LDFLAGS}' $<

bin/api-server: cmd/api-server/main.go $(API_SERVER_SOURCES) $(GO_SOURCES) $(STATIC_SOURCE) $(DEP_SOURCES)
	@go build -o $@ -ldflags='${LDFLAGS}' $<

################
# Docker Build #
################

.PHONY: docker-build docker-build-api-server docker-build-extractor
build-docker: generate
	@docker compose build

build-docker-api-server:
	@docker compose build api-server

build-docker-extractor:
	@docker compose build extractor

##############
# Publishing #
##############

.PHONY: push-docker
push-docker: build-docker
	@docker compose push

###################
# Running Locally #
###################

.PHONY: run-docker run-local run-docker-api-only run-local-apionly
run-docker: apply-devenv run-docker-api-only

run-docker-api-only: build-docker
	@docker compose up api-server

run-local: generate apply-devenv run-local-apionly

run-local-apionly:
	@export OCULAR_CONFIG_PATH="$$(mktemp -d)" && \
	 $(MAKE) generate-api-config-file > $$OCULAR_CONFIG_PATH/config.yaml && \
	 echo "Running API server locally with config at $$OCULAR_CONFIG_PATH/config.yaml" && \
	go run cmd/api-server/main.go

###########################
# Development Environment #
###########################

.PHONY: apply-devenv remove-devenv generate-devenv-token
apply-devenv:
	@./hack/development/dev-env.sh up

apply-devenv-defaults:
	@echo "installing default integrations to the development environment"
	@./hack/development/defaults.sh

remove-devenv:
	@./hack/development/dev-env.sh down

generate-devenv-token:
	@if [ -z "$$DEVENV_TOKEN" ] || ! kubectl auth can-i --token=$$DEVENV_TOKEN get pods > /dev/null 2>&1; then \
		export DEVENV_TOKEN=$$(kubectl create token ${OCULAR_SERVICE_ACCOUNT} --duration 24h); \
	fi; \
	echo $$DEVENV_TOKEN

.PHONY: generate-api-config-file generate-helm-values-yaml apply-ghcr-secret
generate-api-config-file:
	@go run hack/generator/*.go -output - --type api-config

apply-ghcr-secret:
	@./hack/development/imagepullsecrets.sh ghcr

###############
# Development #
###############

.PHONY: generate lint fmt test view-test-coverage fmt-code fmt-license
generate:
	@echo "Generating code ..."
	@OCULAR_LOGGING_LEVEL=error go generate ./...
	@$(MAKE) fmt-license # generated source code files will not have license headers, so we need to run fmt-license after generate

lint:
	@echo "Running linters ..."
	@golangci-lint run ./... --timeout 10m

fmt: generate fmt-code

fmt-license:
	@echo "Formatting license headers ..."
	@docker run --rm -v $(CURRENT_DIR):/github/workspace apache/skywalking-eyes header fix

fmt-code:
	@echo "Running code formatters ..."
	@golangci-lint fmt ./...

test:
	@echo "Running unit tests ..."
	@go test $$(go list ./... | grep -v /internal/unittest) -coverprofile=coverage.out -covermode=atomic

view-test-coverage: test
	@go tool cover -html=coverage.out

serve-docs:
	@command -v godoc > /dev/null 2>&1 || (echo "Please install godoc using 'go install golang.org/x/tools/cmd/godoc@latest'" && exit 1)
	@echo "Serving documentation at http://localhost:6060/pkg/github.com/crashappsec/ocular/"
	@godoc -http=localhost:6060