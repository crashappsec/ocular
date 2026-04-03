#!/bin/sh

SCRIPTS_DIR="/scripts"

source "$SCRIPTS_DIR/common.sh"

finalize() {
    result_file="$OCULAR_RESULTS_DIR/env.test"
    if [ "$?" -eq 0 ]; then
	echo "PASS" > "$OCULAR_RESULTS_DIR/env.test"
    else
	echo "FAIL" > "$OCULAR_RESULTS_DIR/env.test"
    fi
}

trap finalize EXIT

validate-common-env

validate-pwd "$OCULAR_TARGET_DIR"

complete "scanner enviornment validated" 