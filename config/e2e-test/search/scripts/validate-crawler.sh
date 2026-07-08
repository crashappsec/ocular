#!/bin/sh
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.


SCRIPTS_DIR="/scripts"

set -e

source "$SCRIPTS_DIR/common.sh"

validate-env-var "OCULAR_CRAWLER_NAME" "validate-crawler"
validate-env-var "OCULAR_SEARCH_NAME" "e2e-test"
validate-fifo "$OCULAR_PIPELINE_FIFO" 
validate-fifo "$OCULAR_SEARCH_FIFO"
validate-container-name "crawler-validate-container"

validate-parameter "RESOURCE" "CRAWLER"

cat <<EOF >> "$OCULAR_PIPELINE_FIFO"
{
  "identifier": "https://github.com/crashappsec/ocular",
  "version": "1"
}
EOF


cat <<EOF >> "$OCULAR_PIPELINE_FIFO"
{
  "identifier": "https://github.com/crashappsec/chalk",
  "version": "2"
}
EOF


cat <<EOF >> "$OCULAR_PIPELINE_FIFO"
{
  "identifier": "https://github.com/crashappsec/hello-world",
  "version": "3"
}
EOF

complete "crawler completed successfully"