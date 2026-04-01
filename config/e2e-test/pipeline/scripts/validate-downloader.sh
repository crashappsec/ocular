#!/bin/sh

SCRIPTS_DIR="/scripts"

source "$SCRIPTS_DIR/common.sh"



clone-target() {
    info "cloning git target"

    git clone "$OCULAR_TARGET_IDENTIFIER" . || fail "unable to clone repository"
    git checkout "$OCULAR_TARGET_VERSION" || fail "unable to checkout git repository"
    pass "git clone complete"
}

validate-common-env

validate-pwd "$OCULAR_TARGET_DIR"

clone-target

complete "downloader completed successfully"