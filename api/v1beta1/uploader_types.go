// Copyright (C) 2025-2026 Crash Override, Inc.
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

// UploaderSpec defines the desired state of Uploader
type UploaderSpec struct {
	// Container is the container that will be run to download the target.
	// It must be a valid [v1.Container] that can be run in a Kubernetes pod.
	Container v1.Container `json:"container" protobuf:"bytes,1,opt,name=container"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// This list of volumes will be appended to the [k8s.io/api/core/v1.PodSpec] that runs the uploader,
	// which will also include volumes defined by the other Uploader resources defined in the Profile of the Pipeline.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Volumes []v1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,2,rep,name=volumes"`

	// Parameters is a list of ParameterDefinition that can be used to define "parameters"
	// that the user can specify in an uploader reference that can configure how to uploader results.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Parameters []ParameterDefinition `json:"parameters,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,3,rep,name=parameters"`
}

type UploaderStatus struct {
	// Conditions is a list of conditions that the uploader is in.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Uploader's current state."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Uploader is the Schema for the uploaders API
type Uploader struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Uploader
	// +required
	Spec UploaderSpec `json:"spec"`

	// status defines the observed state of Uploader
	// +optional
	Status UploaderStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// UploaderList contains a list of Uploader
type UploaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Uploader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Uploader{}, &UploaderList{})
}
