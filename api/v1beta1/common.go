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

const (
	/*
		The following condition types are used by both Pipeline and Search
		Only one of FailedConditionType or CompleteConditionType should be true at any given time
	*/

	// FailedConditionType indicates that the execution has failed.
	FailedConditionType = "Failed"
	// CompleteConditionType indicates that the execution has completed successfully.
	CompleteConditionType = "Complete"
	// StartedConditionType indicates that the execution has started.
	StartedConditionType = "Started"
)

// Target represents a target to be downloaded by a Downloader.
// The Downloader is responsible for interpreting the Identifier and Version fields,
// and choosing how to represent the target in the filesystem for the Profile to analyze.
type Target struct {
	// Identifier is a unique identifier for the target.
	// This could be a URL, a file path, or any other string that uniquely identifies the target, it
	// is up to the Downloader to interpret this string.
	// +required
	Identifier string `json:"identifier,omitempty" yaml:"identifier,omitempty" description:"A unique identifier for the target. This could be a URL, a file path, or any other string that uniquely identifies the target."`
	// Version is an optional version string for the target.
	// This could be a version number, a commit hash, or any other string that represents the version of the target.
	// It is up to the Downloader to interpret this string.
	// +optional
	Version string `json:"version,omitempty" yaml:"version,omitempty" description:"An optional version string for the target. This could be a version number, a commit hash, or any other string that represents the version of the target."`
}

type ParameterSetting struct {
	// Name is the name of the parameter to set.
	// +required
	Name string `json:"name" yaml:"name" description:"The name of the parameter to set."`
	// Value is the value to set the parameter to.
	// +required
	Value string `json:"value" yaml:"value" description:"The value to set the parameter to."`
}

// ParameterizedObjectReference is a reference to a resource that will be run with parameters.
type ParameterizedObjectReference struct {
	v1.ObjectReference `json:",inline"`

	// Parameters is a list of parameters to pass to the referenced resource.
	// as environment variables.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Parameters []ParameterSetting `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ParameterDefinition is a definition of a parameter that can be passed to a container.
// It defines the name of the parameter, a description of the parameter,
// whether the parameter is required, and a default value for the parameter (when not required).
type ParameterDefinition struct {
	// Name is the name of the parameter.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[a-zA-Z_][a-zA-Z0-9_]*$`
	Name string `json:"name" protobuf:"bytes,1,opt,name=name" yaml:"name" description:"The name of the parameter. Must be a valid environment variable name."`

	// Description is the description of the parameter.
	// +optional
	Description string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description" yaml:"description,omitempty" description:"A description of the parameter."`

	// Required is true if the parameter is required.
	// If true, the execution will fail to start if the parameter is not provided.
	// +required
	Required bool `json:"required" protobuf:"varint,3,opt,name=required" yaml:"required" description:"Whether or not the parameter is required. If true, the execution will fail to start if the parameter is not provided."`

	// Default is the default value for the parameter.
	// It is only valid if Required is false.
	// A null value indicates that if there is no value provided, the environment variable will be unset.
	// +optional
	Default *string `json:"default,omitempty" protobuf:"bytes,4,opt,name=default" yaml:"default,omitempty" description:"The default value for the parameter. It is only valid if Required is false."`
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
