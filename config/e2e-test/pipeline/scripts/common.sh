#!/bin/sh

# directory these scripts are located
# this is not used but serves as a reminder for
# what to hardcode into the source for scripts
SCRIPTS_DIR="/scripts"


pass() {
    echo "PASS: $1"
}

complete() {
    echo "COMPLETE: $1"
    exit 0
}

info() {
    echo "INFO: $1" 1>&2
}

fail() {
    echo "FAIL: $1"
    # custom exit code to debug
    # container exec failures vs
    # script failures
    exit 21
}

validate-env-var() {
    var="$1"
    expected="$2"
    actual="$(env | awk  -F= -v var="$var" '$1 == var {print substr($0, length($1) + 2)}')"

    if [ "$expected" = "$actual" ]; then
	pass "validate environment variable '$var'"
    else
	fail "unexpcted value for '$var' environment variable, got '$actual' expected '$expected'"
    fi
}

validate-pwd() {
    expected="$1"
    actual="$(pwd || fail 'unable to run pwd')"

    if [ "$expected" = "$actual" ]; then
	pass "validated current working directory"
    else
	fail "unexpcted working directory, got '$actual' expected '$expected'"
    fi
}


validate-file-contents() {
    filepath="$1"
    expected="$1"
    actual="$(cat "$filepath" || fail "unable to access file $filepath")"

    if [ "$expected" = "$actual" ]; then
	pass "validated file contents for $filepath"
    else
	fail "unexpcted file contents for $filepath, got '$actual' expected '$expected'"
    fi
}



validate-common-env() {
    validate-env-var "OCULAR_PROFILE_NAME" "validate-scanners"

    validate-env-var "OCULAR_DOWNLOADER_NAME" "git-clone"

    validate-env-var "OCULAR_PIPELINE_NAME" "e2e-test"

    validate-env-var "OCULAR_TARGET_DIR" "/mnt/target"

    validate-env-var "OCULAR_RESULTS_DIR" "/mnt/results"

    validate-env-var "OCULAR_METADATA_DIR" "/mnt/metadata"

    validate-env-var "OCULAR_TARGET_IDENTIFIER" "https://github.com/crashappsec/ocular"

    validate-env-var "OCULAR_TARGET_VERSION" "84462a71dea813105ce746718d7618aeda8923b8"

    validate-env-var "OCULAR_NAMESPACE_NAME" "e2e-test-pipeline"
}