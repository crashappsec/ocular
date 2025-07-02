#!/usr/bin/env bash
# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.


RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

if [[ -z "$1" ]]; then
  echo -e "${RED}Error:${NC} No registry specified. Usage: $0 <registry>"
  echo -e "Available options:"
  echo -e "  ${GREEN}ghcr${NC} (default)"
  echo -e "  ${GREEN}dockerhub${NC}"
  exit 1
fi

REGISTRY="${1:-ghcr}"

echo -e "${YELLOW}WARNING${NC}: this script should never be run on a production cluster."
echo -e "using context '${BLUE}$(kubectl config current-context)${NC}' and namespace '${GREEN}$(kubectl config view --minify -o jsonpath='{..namespace}')${NC}'"
read -erp "Press [Enter] to continue or [Ctrl+C] to cancel..."

case "$REGISTRY" in
  ghcr)
    read -rp "Enter your GitHub username: " GHCR_USERNAME
    read -rsp "Enter your GitHub Personal Access Token: " GHCR_TOKEN
    echo "Creating image pull secret for GitHub Container Registry..."
    kubectl delete secret ghcr-secret --ignore-not-found \
      --namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
    kubectl create secret docker-registry ghcr-secret \
      --docker-server=ghcr.io \
      --docker-username="${GHCR_USERNAME}" \
      --docker-password="${GHCR_TOKEN}" \
      --namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
    echo -e "Secret '${GREEN}ghcr-secret${NC}' created successfully for GitHub Container Registry."
    ;;
  dockerhub)
    echo "Creating image pull secret for Docker Hub..."
    read -rp "Enter your Dockerhub username: " DOCKERHUB_USERNAME
    read -rsp "Enter your Dockerhub Personal Access Token: " DOCKERHUB_TOKEN
    kubectl delete secret dockerhub-secret --ignore-not-found \
      --namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
    kubectl create secret docker-registry dockerhub-secret \
      --docker-server=https://index.docker.io/v1/ \
      --docker-username="${DOCKERHUB_USERNAME}" \
      --docker-password="${DOCKERHUB_TOKEN}" \
      --namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
    echo -e "Secret '${GREEN}dockerhub-secret${NC}' created successfully for GitHub Container Registry."
    ;;
  *)
    echo -e "${RED}Error:${NC} Unsupported registry: $REGISTRY"
    exit 1
    ;;
esac
