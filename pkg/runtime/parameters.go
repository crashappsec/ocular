// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import "os"

// ParameterToEnvironmentVariable converts a parameter name to an environment variable name.
// It converts the name to uppercase, replaces invalid characters with underscores,
// and prefixes it with "OCULAR_PARAM_".
func ParameterToEnvironmentVariable(name string) string {
	result := make([]rune, 0, len(name))
	for _, char := range name {
		nextChar := '_'
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' {
			nextChar = char
		}

		result = append(result, nextChar)
	}
	return "OCULAR_PARAM_" + string(result)
}

// GetParameterFromEnvironment retrieves the value of a parameter from the environment variables.
// It returns the value and a boolean indicating whether the parameter was set.
func GetParameterFromEnvironment(name string) (string, bool) {
	paramName := ParameterToEnvironmentVariable(name)
	value, exists := os.LookupEnv(paramName)
	if !exists {
		return "", false
	}

	return value, true
}
