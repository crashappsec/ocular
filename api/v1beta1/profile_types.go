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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProfileSpec defines the desired state of Profile
type ProfileSpec struct {
	// Containers is a list of [v1.Container] that will be run
	// in parallel, with their current working directory set to
	// the directory where the target has been downloaded to.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	Containers []v1.Container `json:"containers" yaml:"containers" description:"A list of containers that will be run over the target. The containers will be run in parallel, with their current working directory set to the directory where the target has been downloaded to."`
	// Artifacts is a list of paths to the artifacts that will be produced
	// by the scanners. These paths are relative to the results directory
	Artifacts []string `json:"artifacts" yaml:"artifacts" description:"A list of paths to the artifacts that will be produced by the scanners. These paths are relative to the results directory."`
	// Volumes is a list of [v1.Volume] that will be defined in the pod spec
	// for the scanners. This is useful for sharing data between scanners
	Volumes []v1.Volume `json:"volumes,omitempty" yaml:"volumes,omitempty" description:"A list of volumes that will be mounted into the scanner containers. This is useful for sharing data between scanners or for providing configuration files."`

	// UploaderRefs is a list of [UploaderRunSpec] that will be used to upload
	// the results of the scanners. An uploader will be passed each of the artifacts
	// as command line arguments, prefixed by the argument '--' . Each [UploaderRunRef] must specify the
	// name of the uploader and any parameters that are required.
	// +optional
	UploaderRefs []UploaderRunRef `json:"uploaderRefs" yaml:"uploaderRefs" description:"A list of uploaders that will be used to upload the results of the scanners. An uploader will be passed each of the artifacts as command line arguments, prefixed by the argument '--'. Each UploaderRunRequest must specify the name of the uploader and any parameters that are required."`
}

type ProfileStatus struct {
	// Conditions represent the latest available observations of a Profile's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Profile's current state."`
	Valid      bool               `json:"valid" description:"Whether or not the profile is valid."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Profile is the Schema for the profiles API
type Profile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Profile
	// +required
	Spec ProfileSpec `json:"spec"`

	// status defines the current state of Profile
	// +optional
	Status ProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProfileList contains a list of Profile
type ProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Profile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Profile{}, &ProfileList{})
}
