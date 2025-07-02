// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import (
	"github.com/crashappsec/ocular/internal/config"
	"go.uber.org/zap"
)

const (
	// LabelResource is the label used to identify the resource type.
	LabelResource = "resource"
	// LabelID is the label used to identify the resource ID.
	LabelID = "id"
)

// CreateLabels creates a map of labels for the container.
// It merges the labels defined in the configuration file with
// the additional labels provided as input.
func CreateLabels(additional map[string]string) map[string]string {
	merged := make(map[string]string)
	for key, val := range config.State.Runtime.Labels {
		if len(key) == 0 {
			continue
		}
		if len(key) > 63 {
			zap.L().Debug("label key too long, truncating", zap.String("key", key))
			key = key[:63]
		}
		if len(val) > 63 {
			zap.L().Debug("label value too long, truncating",
				zap.String("key", key), zap.String("value", val))
			val = val[:63]
		}
		merged[key] = val
	}
	for k, v := range additional {
		merged[k] = v
	}
	return merged
}
