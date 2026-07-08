#!/bin/sh
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

set -e

SCRIPTS_DIR="/scripts"

source "$SCRIPTS_DIR/common.sh"

validate-common-pipeline-env

validate-pwd "$OCULAR_RESULTS_DIR"

validate-parameter "TEST_PARAM" "testing-param-value"
validate-parameter "DEFAULT" "default-value"

validate-container-name "uploader-validate-env"

validate-env-var "OCULAR_UPLOADER_NAME" "validate-uploader-env"

complete "uploader env completed successfully"
