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

func ShouldInclude(includeIf *v1beta1.ContainerCondition, params []v1beta1.ParameterSetting) bool {
	if includeIf == nil {
		return true
	}

	if expectedParam := includeIf.WhenParamSet; expectedParam != "" {
		for _, param := range params {
			if param.Name == expectedParam && param.Value != "" {
				return true
			}
		}
		return false
	}
	return true
}

func FilterConditionalContainers(cs []v1beta1.ConditionalContainer, params []v1beta1.ParameterSetting) []v1.Container {
	var result []v1.Container
	for _, c := range cs {
		if ShouldInclude(c.IncludeIf, params) {
			result = append(result, c.Container)
		}
	}
	return result
}
