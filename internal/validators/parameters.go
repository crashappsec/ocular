// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package validators

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/api/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func UndefinedParameters(params []v1beta1.ParameterDefinition, paramValues []v1beta1.ParameterSetting) []string {
	var definedParams = make(map[string]struct{}, len(params))
	for _, paramValue := range paramValues {
		definedParams[paramValue.Name] = struct{}{}
	}
	var undefined []string
	for _, param := range params {
		if _, exists := definedParams[param.Name]; !exists {
			undefined = append(undefined, param.Name)
		}
	}
	return undefined
}

func GetNewRequiredParameters(oldParams, newParams []v1beta1.ParameterDefinition) []string {
	result := make([]string, 0, len(oldParams)+len(newParams))
	var newRequiredParameters = make(map[string]v1beta1.ParameterDefinition, len(newParams))
	for _, paramDef := range newParams {
		if paramDef.Default == nil {
			newRequiredParameters[paramDef.Name] = paramDef
		}
	}

	for _, oldParamDef := range oldParams {
		if oldParamDef.Default == nil {
			delete(newRequiredParameters, oldParamDef.Name)
		}
	}

	for paramName := range newRequiredParameters {
		result = append(result, paramName)
	}
	return result
}

func ParseSetParameters(ref v1beta1.ParameterizedObjectReference, definitions []v1beta1.ParameterDefinition) (set []v1beta1.ParameterSetting, unset []v1beta1.ParameterDefinition) {
	var params = make(map[string]v1beta1.ParameterSetting)
	for _, param := range ref.Parameters {
		params[param.Name] = param
	}
	for _, def := range definitions {
		if setting, defined := params[def.Name]; defined {
			set = append(set, setting)
		} else {
			unset = append(unset, def)
		}
	}

	return
}

func ValidateParameterReference(ctx context.Context, refPath *field.Path, ref v1beta1.ParameterizedObjectReference, paramDefs []v1beta1.ParameterDefinition) field.ErrorList {
	var (
		paramErrors field.ErrorList
		setParams   = make(map[string]struct{}, len(ref.Parameters))
	)
	for _, paramSetting := range ref.Parameters {
		setParams[paramSetting.Name] = struct{}{}
	}

	for _, param := range paramDefs {
		if _, ok := setParams[param.Name]; !ok && param.Default == nil {
			paramErrors = append(paramErrors,
				field.Invalid(refPath.Child("parameters"), ref.Parameters, fmt.Sprintf(
					"missing required parameter %s in reference to %s resource %s/%s",
					param.Name, ref.Kind, ref.Namespace, ref.Name,
				)))
		}
	}

	return paramErrors
}
