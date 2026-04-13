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
	ocularRuntime "github.com/crashappsec/ocular/pkg/runtime"
	v1 "k8s.io/api/core/v1"
)

func ParseParameterEnvVars(definitions []v1beta1.ParameterDefinition, settings []v1beta1.ParameterSetting) []v1.EnvVar {
	var params = make(map[string]string)
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
			params[setting.Name] = setting.Value
		}
	}
	env := make([]v1.EnvVar, 0, len(params))
	for param, value := range params {
		env = append(env, v1.EnvVar{
			Name:  ocularRuntime.ParameterToEnvironmentVariable(param),
			Value: value,
		})
	}
	return env
}
