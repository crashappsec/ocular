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

type DownloaderSpec struct {
	// Container is the container that will be run to download the target.
	// It must be a valid [v1.Container] that can be run in a Kubernetes pod.
	// +required
	Container v1.Container `json:"container" yaml:"container" description:"The container that will be run to download the target."`

	// Volumes is a list of volumes that will be appended to the [k8s.io/api/core/v1.PodSpec]
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty" yaml:"volumes,omitempty" description:"A list of volumes that will be mounted into the downloader container. This is useful for sharing data between downloaders or for providing configuration files."`
}

type DownloaderStatus struct {
	// Conditions represent the latest available observations of a Downloader's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Downloader's current state."`

	// Valid indicates whether the downloader is valid.
	// +optional
	Valid *bool `json:"valid,omitempty" description:"Whether or not the downloader is valid."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Downloader is the Schema for the downloaders API
type Downloader struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Downloader
	// +required
	Spec DownloaderSpec `json:"spec"`

	// status defines the observed state of Downloader
	// +optional
	Status DownloaderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DownloaderList contains a list of Downloader
type DownloaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Downloader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Downloader{}, &DownloaderList{})
}
