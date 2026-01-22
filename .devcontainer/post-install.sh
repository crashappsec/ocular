#!/bin/bash
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

set -euo pipefail

echo "Installing Kubebuilder development tools..."

ARCH=$(go env GOARCH)

# Install kind
if ! command -v kind &> /dev/null; then
  curl -Lo ./kind "https://kind.sigs.k8s.io/dl/latest/kind-linux-${ARCH}"
  chmod +x ./kind
  mv ./kind /usr/local/bin/kind
fi

# Install kubebuilder
if ! command -v kubebuilder &> /dev/null; then
  curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/linux/${ARCH}"
  chmod +x kubebuilder
  mv kubebuilder /usr/local/bin/
fi

# Install kubectl
if ! command -v kubectl &> /dev/null; then
  KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
  curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl"
  chmod +x kubectl
  mv kubectl /usr/local/bin/kubectl
fi

# Wait for Docker to be ready
for i in {1..30}; do
  if docker info >/dev/null 2>&1; then
    break
  fi
  if [ $i -eq 30 ]; then
    echo "WARNING: Docker not ready after 30s"
  fi
  sleep 1
done

# Create kind network, ignore errors if exists or conflicts
docker network inspect kind >/dev/null 2>&1 || docker network create kind || true

# Verify installations
echo "Installed versions:"
kind version
kubebuilder version
kubectl version --client
docker --version
go version

echo "DevContainer ready!"
