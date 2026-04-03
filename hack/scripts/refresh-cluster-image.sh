#!/usr/bin/env bash
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.


set -e

IMAGE="$1"

if [ -z "$IMAGE" ]; then
    echo "ERROR: please supply an image as the first CLI parameter" 1>&2
    exit 1
fi

kubectl run refresh --image="$IMAGE" --image-pull-policy=Always -n default
sleep 10
kubectl delete pod refresh -n default

echo "complete" 1>&2