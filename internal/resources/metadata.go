// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"maps"
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

func PropagateMetadata(parentLabels ...map[string]string) map[string]string {
	childLabels := make(map[string]string)
	for _, parent := range parentLabels {
		maps.Copy(childLabels, parent)
	}
	// we want to remove any labels/annotations that may be used by a service
	// to reoncile objects
	maps.DeleteFunc(childLabels, func(k string, _ string) bool {
		return shouldExcludeKey(k)
	})

	childLabels["app.kubernetes.io/managed-by"] = "ocular-controller"
	return childLabels
}
