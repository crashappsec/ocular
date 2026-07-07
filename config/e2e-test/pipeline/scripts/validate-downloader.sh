#!/bin/sh
# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.


SCRIPTS_DIR="/scripts"

source "$SCRIPTS_DIR/common.sh"



clone-target() {
    info "cloning git target"

    git clone "$OCULAR_TARGET_IDENTIFIER" . || fail "unable to clone repository"
    git checkout "$OCULAR_TARGET_VERSION" || fail "unable to checkout git repository"

    echo "$OCULAR_TARGET_VERSION" > "$OCULAR_METADATA_DIR/git.metadata"
    pass "git clone complete"
}

validate-common-env

validate-pwd "$OCULAR_TARGET_DIR"

validate-parameter "TEST_PARAM" "PASS"

validate-container-name "downloader-validate-container"

clone-target

complete "downloader completed successfully"