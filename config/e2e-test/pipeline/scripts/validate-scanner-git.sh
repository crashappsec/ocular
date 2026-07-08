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

finalize() {
    result_file="$OCULAR_RESULTS_DIR/git.test"
    if [ "$?" -eq 0 ]; then
	echo "PASS" > "$result_file"
    else
	echo "FAIL" > "$result_file"
    fi
}

trap finalize EXIT

validate-git() {
    remote_url="$(git remote get-url origin || fail 'unable to get remote origin URL')"
            
    case "$remote_url" in
        *github.com/crashappsec/ocular) pass "validated git remote URL";;
        *) fail "unexpected remote URL, got '$remote_url' instead";;
    esac
    
    commit_sha="$(git rev-parse HEAD || fail 'unable to parse git HEAD')"
    
    if [ "$commit_sha" = "84462a71dea813105ce746718d7618aeda8923b8" ]; then
        pass "git HEAD set to correct commit SHA"
    else
        fail "unexpcted commit SHA for HEAD, got '$commit_sha' instead"
    fi
    
}

validate-common-env

validate-git

validate-container-name "scanner-validate-git"

complete "scanner git test succeeded"
