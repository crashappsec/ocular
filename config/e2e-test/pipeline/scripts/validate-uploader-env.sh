#!/bin/sh

SCRIPTS_DIR="/scripts"

source "$SCRIPTS_DIR/common.sh"

validate-common-env

validate-pwd "$OCULAR_RESULTS_DIR"

validate-env-var "OCULAR_PARAM_TEST_PARAM" "testing-param-value"
validate-env-var "OCULAR_PARAM_DEFAULT" "default-value"
validate-env-var "OCULAR_UPLOADER_NAME" "validate-uploader-env"

complete "uploader env completed successfully"
