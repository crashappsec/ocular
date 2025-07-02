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

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

set -e

ctx="$(kubectl config current-context)"
set_namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
namespace="${set_namespace:-default}"

echo -e "${YELLOW}WARNING${NC}: this script should never be run on a production cluster."
echo -e "run 'make devenv-down' to remove created resources"
echo -e "using context '${BLUE}${ctx}${NC}' and namespace '${GREEN}${namespace}${NC}'"
read -erp "Press [Enter] to continue or [Ctrl+C] to cancel..."


manifest="$(cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${OCULAR_PROFILE_CONFIGMAPNAME:-ocular-profiles}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${OCULAR_DOWNLOADERS_CONFIGMAPNAME:-ocular-downloaders}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${OCULAR_CRAWLERS_CONFIGMAPNAME:-ocular-crawlers}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${OCULAR_UPLOADERS_CONFIGMAPNAME:-ocular-uploaders}
---
apiVersion: v1
kind: Secret
metadata:
  name: ${OCULAR_SECRETS_SECRETNAME:-ocular-secrets}
type: Opaque
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ${OCULAR_SERVICE_ACCOUNT_ROLE:-ocular-admin-role}
rules:
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["create", "list", "watch", "get", "delete"]
- apiGroups: [""]
  resources: ["secrets", "configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${OCULAR_SERVICE_ACCOUNT:-ocular-admin}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${OCULAR_SERVICE_ACCOUNT_ROLE_BINDING:-ocular-admin-binding}
subjects:
  - kind: ServiceAccount
    name: ${OCULAR_SERVICE_ACCOUNT:-ocular-admin}
roleRef:
  kind: Role
  name: ${OCULAR_SERVICE_ACCOUNT_ROLE:-ocular-admin-role}
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: default-pod-quota
spec:
  hard:
    pods: "25"
EOF
)"



cmd="$1"

case $cmd in
  up)
    # Create the resources
    echo "$manifest" | kubectl apply -f -
    ;;
  down)
     # Delete the resoureces
     echo "$manifest" | kubectl delete --ignore-not-found=true -f -
    ;;
  *)
    echo "${RED}ERROR${NC}: unknown command '${cmd}'"
    exit 1
    ;;
esac

echo -e "${GREEN}done!${NC} run 'make generate-devenv-token' to get a token for the service account"
