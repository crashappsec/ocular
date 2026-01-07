#!/usr/bin/env bash
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.


set -e

INCLUDE_DEFAULTS=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -i|--include-defaults)
      INCLUDE_DEFAULTS=1
      shift # past argument
      shift # past value
      ;;
    *)
      shift # past argument
      ;;
  esac
done

if ! command -v kubectl &> /dev/null; then
  echo "kubectl could not be found, please install it to run this script"
  exit 1
fi

# check if jq is installed
if ! command -v jq &> /dev/null; then
  echo "jq could not be found, please install it to run this script"
  exit 1
fi

OUTPUT_DIR="${1:-./out/}"
KUBECTX=$(kubectl config current-context)
SELECTED_NAMESPACE=$(kubectl config view --minify --output 'jsonpath={..namespace}')
NAMESPACE=${SELECTED_NAMESPACE:-default}
echo -e "This script will use kubectl context '$(tput bold)\e[31m${KUBECTX}\e[0m$(tput sgr0)' and namespace '$(tput bold)\e[33m${NAMESPACE}\e[0m$(tput sgr0)'"
echo "Make sure this is the correct context and namespace where Ocular is installed."
echo "Press enter to continue or ctrl+c to abort"
read -r
if [ ! -d "$OUTPUT_DIR" ]; then
  mkdir -p "$OUTPUT_DIR"
fi

echo "writing output manfiests to $OUTPUT_DIR"

# This will fail if one of the config maps doesnt exist and write an error message to stderr
kubectl get configmaps ocular-crawlers ocular-uploaders ocular-downloaders ocular-profiles 1>/dev/null

migrate_crawlers() {
  echo "migrating crawlers.."
  CRAWLER_DIR="$OUTPUT_DIR/crawlers"
  mkdir -p "$CRAWLER_DIR"
  declare -A crawlers

  while read -r line; do
    key="${line%%=*}"
    value="${line#*=}"
    crawlers["$key"]="$value"
  done < <(kubectl get configmap ocular-crawlers --output json  | jq -r '.data | to_entries | map("\(.key)=\(.value|@base64)")|.[]')

  for key in "${!crawlers[@]}"; do
    echo "Migrating crawler: $key"
    decoded_value=$(echo "${crawlers[$key]}" | base64 --decode)
    if [ $INCLUDE_DEFAULTS -eq 0 ]; then
      if [[ $(echo "$decoded_value" | yq '.image' -r ) =~ ^ghcr.io/crashappsec/ocular-default-crawlers.*$ ]]; then
        echo "Skipping default crawler: $key"
        continue
      fi
    fi



    cat << EOF | grep -v 'null$' --color=none > "$CRAWLER_DIR/$key.yaml"
apiVersion: ocular.crashoverride.run/v1beta1
kind: Crawler
metadata:
  name: ${key%.*}
  namespace: $NAMESPACE
spec:
  container:
    name: "${key%.*}"
    image: "$(echo "$decoded_value" | yq '.image')"
    imagePullPolicy: "$(echo "$decoded_value" | yq '.imagePullPolicy // "IfNotPresent"')"
    command: $(echo "$decoded_value" | yq '.command // null')
    args: $(echo "$decoded_value" | yq '.args // null')
  parameters: []
  volumes:
    - name: uploader-${key%.*}-secrets
      secret:
        optional: true
        secretName: ocular-user-secrets
EOF
    PARAMETERS="$decoded_value" yq -ie '.spec.parameters += (strenv(PARAMETERS) | from_yaml | .parameters | to_entries | with(.[]; .value += {"name": .key} | . |= .value ))' "$CRAWLER_DIR/$key.yaml"
  done
  echo "crawler migration complete"
}


migrate_uploaders() {
  echo "migrating uploaders.."
  UPLOADER_DIR="$OUTPUT_DIR/uploaders"
  mkdir -p "$UPLOADER_DIR"
  declare -A uploaders

  while read -r line; do
    key="${line%%=*}"
    value="${line#*=}"
    uploaders["$key"]="$value"
  done < <(kubectl get configmap ocular-uploaders --output json  | jq -r '.data | to_entries | map("\(.key)=\(.value|@base64)")|.[]')

  for key in "${!uploaders[@]}"; do
    echo "Migrating uploader: $key"
    decoded_value=$(echo "${uploaders[$key]}" | base64 --decode)
    if [ $INCLUDE_DEFAULTS -eq 0 ]; then
      if [[ "$(echo "$decoded_value" | yq '.image' -r )" =~ ^ghcr.io/crashappsec/ocular-default-uploaders.*$ ]]; then
        echo "Skipping default uploader: $key"
        continue
      fi
    fi


    cat << EOF | grep -v 'null$' --color=none > "$UPLOADER_DIR/$key.yaml"
apiVersion: ocular.crashoverride.run/v1beta1
kind: Uploader
metadata:
  name: ${key%.*}
  namespace: $NAMESPACE
spec:
  container:
    name: "${key%.*}"
    image: "$(echo "$decoded_value" | yq '.image')"
    imagePullPolicy: "$(echo "$decoded_value" | yq '.imagePullPolicy // "IfNotPresent"')"
    command: $(echo "$decoded_value" | yq '.command // null')
    args: $(echo "$decoded_value" | yq '.args // null')
  parameters: []
  volumes:
    - name: uploader-${key%.*}-secrets
      secret:
        optional: true
        secretName: ocular-user-secrets
EOF
    PARAMETERS="$decoded_value" yq -ie '.spec.parameters += (strenv(PARAMETERS) | from_yaml | .parameters | to_entries | with(.[]; .value += {"name": .key} | . |= .value ))' "$UPLOADER_DIR/$key.yaml"
  done
  echo "uploader migration complete"
}


migrate_downloaders() {
  echo "migrating downloaders.."
  DOWNLOADER_DIR="$OUTPUT_DIR/downloaders"
  mkdir -p "$DOWNLOADER_DIR"
  declare -A downloaders

  while read -r line; do
    key="${line%%=*}"
    value="${line#*=}"
    downloaders["$key"]="$value"
  done < <(kubectl get configmap ocular-downloaders --output json  | jq -r '.data | to_entries | map("\(.key)=\(.value|@base64)")|.[]')

  for key in "${!downloaders[@]}"; do
    echo "Migrating downloader: $key"
    decoded_value=$(echo "${downloaders[$key]}" | base64 --decode)
    if [ $INCLUDE_DEFAULTS -eq 0 ]; then
      if [[ "$(echo "$decoded_value" | yq '.image' -r )" =~ ^ghcr.io/crashappsec/ocular-default-downloaders.*$  ]]; then
        echo "Skipping default downloader: $key"
        continue
      fi
    fi

    cat << EOF | grep -v 'null$' --color=none > "$DOWNLOADER_DIR/$key.yaml"
apiVersion: ocular.crashoverride.run/v1beta1
kind: Downloader
metadata:
  name: ${key%.*}
  namespace: $NAMESPACE
spec:
  container:
    name: "${key%.*}"
    image: "$(echo "$decoded_value" | yq '.image')"
    imagePullPolicy: "$(echo "$decoded_value" | yq '.imagePullPolicy // "IfNotPresent"')"
    command: $(echo "$decoded_value" | yq '.command // null')
    args: $(echo "$decoded_value" | yq '.args // null')
EOF
  done
  echo "downloader migration complete"
}

migrate_secrets() {
  echo "migrating secrets.."
  SECRET_DIR="$OUTPUT_DIR/secrets"
  mkdir -p "$SECRET_DIR"

  export NAMESPACE
  kubectl get -o YAML secret ocular-secrets -n "$NAMESPACE" | yq e 'del(.metadata) | .metadata.name = "ocular-user-secrets" | .metadata.namespace = strenv(NAMESPACE)' > "$SECRET_DIR/ocular-user-secrets.yaml"
  echo "secret migration complete"
}

migrate_profiles() {
  echo "migrating profiles.."
  PROFILE_DIR="$OUTPUT_DIR/profiles"
  mkdir -p "$PROFILE_DIR"
  declare -A profiles

  while read -r line; do
    key="${line%%=*}"
    value="${line#*=}"
    profiles["$key"]="$value"
  done < <(kubectl get configmap ocular-profiles --output json  | jq -r '.data | to_entries | map("\(.key)=\(.value|@base64)")|.[]')

  for key in "${!profiles[@]}"; do
    echo "Migrating profile: $key"
    decoded_value=$(echo "${profiles[$key]}" | base64 --decode)

    declare -a scanners
    scanner_num=0
    echo "migrating scanners"
    while read -r -u 3 -d $'\x01' doc; do
      IFS= read -r -d '' formatted <<EOF || true
name: "scanner-${scanner_num}"
image: "$(yq '.image' <<< "$doc")"
imagePullPolicy: "$(yq '.imagePullPolicy // "IfNotPresent"' <<< "$doc")"
command: $(yq '.command // null' <<< "$doc")
args: $(yq '.args // null' <<< "$doc")
EOF
      scanners+=("$formatted")
      ((scanner_num+=1))
    done 3< <(echo "$decoded_value" | yq e '.scanners[] | split_doc'  | sed 's/---/\x01/g' | cat - <(echo $'\x01'))

    echo "creating file"
    cat << EOF | grep -v 'null$' --color=none > "$PROFILE_DIR/$key.yaml"
apiVersion: ocular.crashoverride.run/v1beta1
kind: Profile
metadata:
  name: ${key%.*}
  namespace: $NAMESPACE
spec:
  containers: []
  artifacts: []
  uploaderRefs: []
EOF
    ARTIFACTS="$decoded_value" yq -ie '.spec.artifacts += (strenv(ARTIFACTS) | from_yaml | .artifacts)' "$PROFILE_DIR/$key.yaml"

    for scanner in "${scanners[@]}"; do
      SCANNER="$scanner" yq -ie '.spec.containers += (strenv(SCANNER) | from_yaml)' "$PROFILE_DIR/$key.yaml"
    done

    while read -u 4 -r -d $'\x01' uploader; do
      UPLOADER="$uploader" yq -ie '.spec.uploaderRefs += (strenv(UPLOADER) | from_yaml | .parameters = (.parameters | to_entries | with(.[]; . += {"name": .key} | del(.key))) | [.])' "$PROFILE_DIR/$key.yaml"
    done 4< <(echo "$decoded_value" | yq e '.uploaders[] | split_doc'  | sed 's/---/\x01/g' | cat - <(echo $'\x01'))
  done
  echo "profile migration complete"
}

migrate_crawlers
migrate_uploaders
migrate_downloaders
migrate_profiles

migrate_secrets

echo "migration complete!"
echo "You can now apply the new manifests with 'kubectl apply -f $OUTPUT_DIR --namespace $NAMESPACE'"