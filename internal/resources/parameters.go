// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"github.com/crashappsec/ocular/api/v1beta1"
)

func ParseParameters(
	definitions []v1beta1.ParameterDefinition,
	settings []v1beta1.ParameterSetting,
	parentSettings map[string]string,
) map[string]string {
	var params = make(map[string]string, len(definitions))
	// Set parameters
	for _, def := range definitions {
		if def.Default != nil {
			params[def.Name] = *def.Default
		} else {
			params[def.Name] = ""
		}
	}

	// Set defaults for missing
	for _, setting := range settings {
		// filter out params that are not specified in the definitions
		if _, exists := params[setting.Name]; exists {
			if setting.ValueFrom != nil && parentSettings != nil {
				params[setting.Name] = parentSettings[setting.ValueFrom.ParentParam]
			} else {
				params[setting.Name] = setting.Value
			}
		}
	}
	return params
}
