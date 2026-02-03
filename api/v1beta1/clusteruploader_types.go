// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterUploaderStatus defines the observed state of ClusterUploader.
type ClusterUploaderStatus struct {
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterUploader is the Schema for the clusteruploaders API
type ClusterUploader struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterUploader
	// This is the same as [UploaderSpec].
	// +required
	Spec UploaderSpec `json:"spec"`

	// status defines the observed state of ClusterUploader
	// +optional
	Status ClusterUploaderStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterUploaderList contains a list of ClusterUploader
type ClusterUploaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterUploader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterUploader{}, &ClusterUploaderList{})
}
