// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package config

import (
	"strings"
)

// Environment represents the different environments the application can run in.
// It is used to determine the current environment based on the configuration file.
type Environment = uint8

const (
	// EnvProduction represents the production environment.
	EnvProduction Environment = iota
	// EnvStaging represents the staging environment. This
	// ideally should be result in a configuration identical to production,
	// but mainly used for metadata purposes.
	EnvStaging
	// EnvDevelopment represents the development environment.
	EnvDevelopment
	// EnvTest represents the test environment (both unit and integration).
	EnvTest
)

var currentEnv Environment

// InitEnv initializes the environment based on the configuration file.
func InitEnv() {
	switch strings.ToLower(State.Environment) {
	case "staging":
		currentEnv = EnvStaging
	case "development":
		currentEnv = EnvDevelopment
	case "test":
		currentEnv = EnvTest
	case "production":
		fallthrough
	default:
		currentEnv = EnvProduction
	}
}

// IsEnvironmentIn checks if the current environment is one of the provided environments.
func IsEnvironmentIn(envs ...Environment) bool {
	for _, env := range envs {
		if env == currentEnv {
			return true
		}
	}
	return false
}
