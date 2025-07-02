// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

import (
	"regexp"
	"strings"
)

// UserContainer represents a user defined container that will be
// run by the application. It is a subset and simplified version of [k8s.io/api/core/v1.Container].
type UserContainer struct {
	Image           string   `json:"image"                     yaml:"image"                     mapstructure:"image"`
	ImagePullPolicy string   `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty" mapstructure:"imagePullPolicy"`
	Command         []string `json:"command,omitempty"         yaml:"command,omitempty,flow"    mapstructure:"command"`
	Args            []string `json:"args,omitempty"            yaml:"args,omitempty,flow"       mapstructure:"args"`

	Secrets []SecretRef `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Env     []EnvVar    `json:"env,omitempty"     yaml:"env,omitempty"`
}

// UserContainerWithParameters is a wrapper around the UserContainer type
// that additionally defines a set of parameters that can be passed to the container.
// There parameters will be passed to the container as environment variables.
// During the API call that invokes the container, the user should pass the values
// for the parameters as a map of strings. Parameters that are required should be
// validated for existence. See [ParameterDefinition] for more information on defining
// parameters.
type UserContainerWithParameters struct {
	UserContainer `                               yaml:",inline"`
	Parameters    map[string]ParameterDefinition `yaml:"parameters,omitempty" json:"parameters,omitempty" description:"Parameters that can be passed to the container. They will be passed as environment variables." pattern:"^[A-Z][A-Z0-9_]*$"`
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

// ParameterNameToEnv converts a parameter name to the environment variable name
// it would be passed as in the container
func ParameterNameToEnv(name string) string {
	return ParamEnvVarPrefix + FormatParamName(name)
}

// EnvToParameterName converts an environment variable name to the parameter name
func EnvToParameterName(name string) string {
	return strings.TrimPrefix(name, ParamEnvVarPrefix)
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// FormatParamName formats a parameter name to be used as an environment variable.
// It replaces all non-alphanumeric characters with underscores
func FormatParamName(name string) string {
	// Replace all non-alphanumeric characters with underscores
	validCharsName := nonAlphanumericRegex.ReplaceAllString(name, "_")
	// Convert to uppercase
	envFormatName := strings.ToUpper(strings.ReplaceAll(validCharsName, "-", "_"))
	return envFormatName
}

// SecretMountType represents the type of secret mount.
// it can be either an environment variable ([SecretMountTypeEnvVar])
// or a file ([SecretMountTypeFile]).
type SecretMountType = string

const (
	// SecretMountTypeEnvVar is used to mount a secret as an environment variable.
	SecretMountTypeEnvVar SecretMountType = "envVar"
	// SecretMountTypeFile is used to mount a secret as a file.
	SecretMountTypeFile SecretMountType = "file"
)

// IsValidSecretMount checks if the given mount type is valid.
func IsValidSecretMount(mountType SecretMountType) bool {
	switch mountType {
	case SecretMountTypeEnvVar, SecretMountTypeFile:
		return true
	default:
		return false
	}
}

// SecretRef represents a reference to a secret.
// It should define the secret name and how to mount it.
// If a secret is marked required, the application will fail to start
// or define containers that reference it if the secret is not found.
type SecretRef struct {
	Name        string          `json:"name"                  yaml:"name"                  description:"Name of the secret to reference."`
	MountType   SecretMountType `json:"mountType,omitempty"   yaml:"mountType,omitempty"   description:"Type of mount for the secret, either envVar or file."                                                                                                                           enum:"envVar,file"`
	MountTarget string          `json:"mountTarget,omitempty" yaml:"mountTarget,omitempty" description:"Target path for the secret mount. If mountType is envVar, this is the environment variable name. If mountType is file, this is the file path where the secret will be mounted."`
	Required    bool            `json:"required,omitempty"    yaml:"required,omitempty"    description:"Whether the secret is required. If true, the application will fail to start if the secret is not found."`
}

// EnvVar represents an environment variable.
type EnvVar struct {
	Name  string `json:"name,omitempty"  yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}
