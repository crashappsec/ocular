#!/bin/bash
# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

# This script creates the default resources in the ocular via the API

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

set -e

ctx="$(kubectl config current-context)"
set_namespace="$(kubectl config view --minify -o jsonpath='{..namespace}')"
namespace="${set_namespace:-default}"

echo -e "${YELLOW}WARNING${NC}: this script should never be run on a production cluster."
echo -e "using context '${BLUE}${ctx}${NC}' and namespace '${GREEN}${namespace}${NC}'"
read -erp "Press [Enter] to continue or [Ctrl+C] to cancel..."

release_version="${1:-latest}"

required_commands=(jq unzip yq)
for cmd in "${required_commands[@]}"; do
	if ! command -v "$cmd" >/dev/null; then
		echo "'$cmd' not detected, please ensure it is installed and located in your \$PATH"
	fi
done

if [[ ! "$(yq --help)" =~ "github.com/mikefarah/yq" ]]; then
	echo "wrong version of 'yq' detected, please use https://github.com/mikefarah/yq"
fi

download_release() {
	identifier="$1"

	output_folder="$(mktemp -d -t ocular-default-integrations)"
	zip_path="$(mktemp -t ocular-default-integrations-zip)"

	api_url="https://api.github.com/repos/crashappsec/ocular-default-integrations/releases/${identifier}"

	download_url="$(
		curl \
			-H "Authorization: Bearer $GITHUB_TOKEN" \
			-H "X-GitHub-Api-Version: 2022-11-28" \
			-H "Accept: application/vnd.github+json" \
			-fsSL "${api_url}" |
			jq -r '.assets []| select(.name | startswith("ocular-default-integrations-definitions-")) | .url'
	)"

	# Have to set 'application/octet-stream' in order to get raw asset
	# and not metadata
	curl -fsSL \
		-H "Authorization: Bearer $GITHUB_TOKEN" \
		-H "Accept: application/octet-stream" \
		"$download_url" -o "$zip_path"

	unzip -qq "$zip_path" -d "$output_folder"

	rm -f "$zip_path"

	echo "$output_folder"
}

release_identifier="latest"

shopt -s extglob # we have to enable extglob for the following regex patterns to work
case "$release_version" in
latest)
	echo
	;;
v+([0-9]).+([0-9]).+([0-9])?(-@(alpha|beta|rc).+([0-9])))
	release_identifier="tags/$release_version"
	;;
+([0-9]).+([0-9]).+([0-9])?(-@(alpha|beta|rc).+([0-9])))
	release_identifier="tag/v$release_version"
	;;
*)
	echo "invalid version '$release_version', should be of the form 'vX.X.X' or 'latest'" 1>&2
	exit 1
	;;
esac

definitions=$(download_release "$release_identifier")

for resource_definitions in "$definitions"/*; do
	resource_defaults=""
	resource_type=$(basename "$resource_definitions")
	for def in "$resource_definitions"/*; do
		resource_name=$(basename "$def")
		resource_defaults="$resource_defaults\n${resource_name%.*}: |\n$(cat "$def" | sed 's/^/  /')\n"
	done

	kubectl patch "configmap/ocular-$resource_type" \
		-n kube-system \
		--type merge \
		-p "{\"data\":{\"$resource_name\":\"\"}}"
done
