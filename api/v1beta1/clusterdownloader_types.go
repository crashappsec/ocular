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

// ClusterDownloaderStatus defines the observed state of ClusterDownloader.
type ClusterDownloaderStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions represent the current state of the ClusterDownloader resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterDownloader is the Schema for the clusterdownloaders API
type ClusterDownloader struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterDownloader
	// It is the same as [DownloaderSpec]
	// +required
	Spec DownloaderSpec `json:"spec"`

	// status defines the observed state of ClusterDownloader
	// +optional
	Status ClusterDownloaderStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterDownloaderList contains a list of ClusterDownloader
type ClusterDownloaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterDownloader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDownloader{}, &ClusterDownloaderList{})
}
