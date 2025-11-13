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
	Container v1.Container `json:"container" protobuf:"bytes,1,opt,name=container"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Volumes []v1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,2,rep,name=volumes"`

	// MetadataFiles is a list of metadata files that the downloader will produce
	// alongside the downloaded target. These files can contain additional information
	// about the download process, such as checksums, download timestamps, or source URLs.
	// +optional
	// +kubebuilder:validation:MinItems=0
	// +kubebuilder:validation:MaxItems=10
	// +listType=set
	MetadataFiles []string `json:"metadataFiles,omitempty" protobuf:"bytes,3,opt,name=metadataFiles" patchStrategy:"merge"`
}

type DownloaderStatus struct {
	// Conditions represent the latest available observations of a Downloader's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Downloader's current state."`
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
