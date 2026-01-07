// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package validators

import ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"

func AllParametersDefined(paramNames []string, paramValues []ocularcrashoverriderunv1beta1.ParameterSetting) bool {
	var definedParams = make(map[string]struct{}, len(paramNames))
	for _, paramValue := range paramValues {
		definedParams[paramValue.Name] = struct{}{}
	}
	for _, paramName := range paramNames {
		if _, exists := definedParams[paramName]; !exists {
			return false
		}
	}
	return true
}

func GetNewRequiredParameters(oldParams, newParams []ocularcrashoverriderunv1beta1.ParameterDefinition) []string {
	result := make([]string, 0, len(oldParams)+len(newParams))
	var newRequiredParameters = make(map[string]ocularcrashoverriderunv1beta1.ParameterDefinition, len(newParams))
	for _, paramDef := range newParams {
		if paramDef.Required {
			newRequiredParameters[paramDef.Name] = paramDef
		}
	}

	for _, oldParamDef := range oldParams {
		if oldParamDef.Required {
			delete(newRequiredParameters, oldParamDef.Name)
		}
	}

	for paramName := range newRequiredParameters {
		result = append(result, paramName)
	}
	return result
}
