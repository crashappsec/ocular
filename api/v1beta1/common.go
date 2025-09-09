// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	v1 "k8s.io/api/core/v1"
)

type Target struct {
	Identifier string `json:"identifier,omitempty" yaml:"identifier,omitempty" description:"A unique identifier for the target. This could be a URL, a file path, or any other string that uniquely identifies the target."`
	Version    string `json:"version,omitempty" yaml:"version,omitempty" description:"An optional version string for the target. This could be a version number, a commit hash, or any other string that represents the version of the target."`
}

type ParameterizedRunRef struct {
	// Name is the name of the resource that will be run.
	Name string `json:"name" yaml:"name" description:"The name of the resource that will be run."`
	// Parameters is a map of parameters that will be passed to the resource.
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ParameterDefinition is a definition of a parameter that can be passed to a container.
// It defines the name of the parameter, a description of the parameter,
// whether the parameter is required, and a default value for the parameter (when not required).
type ParameterDefinition struct {
	// Description is the description of the parameter.
	Description string `json:"description,omitempty" yaml:"description,omitempty" description:"Description of the parameter."`
	// Required is true if the parameter is required.
	Required bool `json:"required"              yaml:"required"              description:"Will mark the parameter as required. If true, the executuon will fail to start if the parameter is not provided."`
	// Default is the default value for the parameter.
	// It is only valid if Required is false.
	Default string `json:"default,omitempty"     yaml:"default,omitempty"`
}

// ParameterToEnvironmentVariable converts a parameter name to an environment variable name.
// It converts the name to uppercase, replaces invalid characters with underscores,
// and prefixes it with "OCULAR_PARAM_".
func ParameterToEnvironmentVariable(name string) string {
	result := make([]rune, 0)
	for _, char := range name {
		nextChar := '_'
		if char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}

		if char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' {
			nextChar = char
		}

		result = append(result, nextChar)
	}
	return "OCULAR_PARAM_" + string(result)
}

type ServiceAccountDefinition struct {
	// Name is the name of the service account.
	// +required
	Name string `json:"name" yaml:"name" description:"The name of the service account that will be used to run the resource."`

	// Namespace is the namespace of the service account.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty" description:"The namespace of the service account. If not specified, the resource will be run in the same namespace as the resource itself."`

	// IgnoreMissing indicates whether the service account should be ignored if it is not found.
	// +optional
	IgnoreMissing bool `json:"ignoreMissing,omitempty" yaml:"ignoreMissing,omitempty" description:"If true, the service account will be ignored if it is not found. If false, the resource will fail to start if the service account is not found."`

	// TokenProjection is the projection of the service account token that will be mounted into the pod.
	// +optional
	Token v1.ServiceAccountTokenProjection `json:"token,omitempty" yaml:"token,omitempty" description:"The projection of the service account token that will be mounted into the pod. If not specified, the token will not be mounted."`
}
