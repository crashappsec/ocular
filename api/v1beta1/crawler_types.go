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

type CrawlerRunRef = ParameterizedObjectReference

type CrawlerStatus struct {
	// Conditions is a list of conditions that the crawler is in.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Uploader's current state."`
}

// CrawlerSpec defines the desired state of Crawler
type CrawlerSpec struct {
	// Container is the container that will be run to download the target.
	// It must be a valid [v1.Container] that can be run in a Kubernetes pod.
	// +required
	Container v1.Container `json:"container" yaml:"container" description:"The container that will be run to enumerate targets and create Pipelines."`

	// List of volumes that can be mounted by containers belonging to the pod.
	// This list of volumes will be appended to the [k8s.io/api/core/v1.PodSpec] that runs the crawler,
	// which will also include volumes defined by the other Uploader resources defined in the Profile of the Pipeline.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Volumes []v1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,2,rep,name=volumes"`

	// Parameters is a list of ParameterDefinition that can be used to define user enter "parameters"
	// that the crawler can use to configure how to crawl targets. The crawler can use these parameters
	// to customize its behavior. The parameters can be used in the crawler's command line
	// arguments, environment variables, or any other way that the crawler supports.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Parameters []ParameterDefinition `json:"parameters,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,3,rep,name=parameters"`

	// AdditionalPodMetadata defines additional specifications to be added to the pod
	// running the scanners, such as annotations and labels.
	// +optional
	AdditionalPodMetadata AdditionalPodMetadata `json:"podSpecAdditions,omitempty" yaml:"podSpecAdditions,omitempty" description:"Additional specifications to be added to the pod running the crawler, such as annotations and labels."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Crawler is the Schema for the crawlers API
type Crawler struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Crawler
	// +required
	Spec CrawlerSpec `json:"spec"`

	// status defines the observed state of Crawler
	// +optional
	Status CrawlerStatus `json:"status,omitempty,omitzero"`
}

type CrawlerObjectReference = ParameterizedObjectReference

// +kubebuilder:object:root=true

// CrawlerList contains a list of Crawler
type CrawlerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Crawler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Crawler{}, &CrawlerList{})
}
