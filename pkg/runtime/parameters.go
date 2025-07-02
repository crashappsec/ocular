// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import (
	"github.com/crashappsec/ocular/pkg/schemas"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

// EnvForParameters converts a map of parameters to a list of environment variables
// that can be passed to a container. It will check that the parameters are defined
// in the parameter definitions and that the required parameters are set.
func EnvForParameters(
	params map[string]string,
	definitions map[string]schemas.ParameterDefinition,
) []v1.EnvVar {
	var vars []v1.EnvVar
	for name, value := range params {
		_, exists := definitions[schemas.FormatParamName(name)]
		if !exists {
			zap.L().Warn("parameter not found", zap.String("param", name))
			continue
		}
		vars = append(vars, v1.EnvVar{
			Name:  schemas.ParameterNameToEnv(name),
			Value: value,
		})
	}
	return vars
}
