// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package containers

import (
	"github.com/crashappsec/ocular/api/v1beta1"
	v1 "k8s.io/api/core/v1"
)

func shouldInclude(includeIf *v1beta1.ContainerCondition, setParams map[string]bool) bool {
	if includeIf == nil {
		return true
	}

	if expectedParam := includeIf.WhenParamSet; expectedParam != "" {
		return setParams[expectedParam]
	}

	return true
}

func FilterConditionalContainers(cs []v1beta1.ConditionalContainer, definitions []v1beta1.ParameterDefinition, settings []v1beta1.ParameterSetting) []v1.Container {
	var result []v1.Container

	var setParams = make(map[string]bool)
	// Set parameters
	for _, def := range definitions {
		if def.Default != nil && *def.Default != "" {
			setParams[def.Name] = true
		} else {
			setParams[def.Name] = false
		}
	}

	// Set defaults for missing
	for _, setting := range settings {
		// filter out params that are not specified in the definitions
		if _, exists := setParams[setting.Name]; exists {
			setParams[setting.Name] = setting.Value != ""
		}
	}

	for _, c := range cs {
		if shouldInclude(c.IncludeIf, setParams) {
			result = append(result, c.Container)
		}
	}
	return result
}
