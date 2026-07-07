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


FOUND_GIT_TEST=false
GIT_TEST_PATH="$OCULAR_RESULTS_DIR/git.test"
FOUND_ENV_TEST=false
ENV_TEST_PATH="$OCULAR_RESULTS_DIR/env.test"
FOUND_GIT_METADATA=false
GIT_METADATA_PATH="$OCULAR_METADATA_DIR/git.metadata"

if [ "$1" != "--" ]; then
    fail "-- not first argument"
fi
shift 1

if [ "$#" -ne 3 ]; then
    fail "expected 3 arguments, got $#"
fi

while [ "$#" -gt 0 ]; do
    echo "checking file $1"
    case "$1" in
        $GIT_TEST_PATH)
            if $FOUND_GIT_TEST; then fail "duplicate entry for git test"; fi
            FOUND_GIT_TEST=true

	    validate-file-contents "$GIT_TEST_PATH" "PASS"
            ;;
        $ENV_TEST_PATH)
            if $FOUND_ENV_TEST; then fail "duplicate entry for env test"; else FOUND_ENV_TEST=true; fi

	    validate-file-contents "$ENV_TEST_PATH" "PASS"
            ;;
        $GIT_METADATA_PATH)
            if $FOUND_GIT_METADATA; then fail "duplicate entry for env test"; fi
            FOUND_GIT_METADATA=true
	    validate-file-contents "$GIT_METADATA_PATH" "84462a71dea813105ce746718d7618aeda8923b8"
            ;;
        *)
            fail "unknown argument given via CLI: $1";;
    esac
    shift 1
done




validate-common-env

validate-pwd "$OCULAR_RESULTS_DIR"

validate-env-var "OCULAR_UPLOADER_NAME" "validate-uploader-files"

validate-container-name "uploader-validate-files"

complete "uploader files completed successfully"