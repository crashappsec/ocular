// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"strings"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// excludedMetadataKeyPrefixes is the list of
// prefixes for keys that should not be propageted
// from one resource to another
var excludedMetadataKeyPrefixes = []string{
	"kubectl.kubernetes.io/",
	"app.kubernetes.io/",
	"helm.sh/",
	v1beta1.Group + "/",
}

func shouldExcludeKey(key string) bool {
	for _, prefix := range excludedMetadataKeyPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func PropagateMetadata(src map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range src {
		if !shouldExcludeKey(k) {
			result[k] = v
		}
	}
	return result
}
