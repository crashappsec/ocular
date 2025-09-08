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

// SearchSpec defines the desired state of Search
type SearchSpec struct {
	// CrawlerRef is a reference to the crawler that will be run in this search.
	// It should point to a valid Crawler resource in the same namespace.
	// +required
	CrawlerRef string `json:"crawlerRef,omitempty" protobuf:"bytes,1,opt,name=crawlerRef"`

	// Parameters is a map of parameters that will be passed to the crawler.
	// These parameters should be defined in the referenced Crawler spec.
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// TTLSecondsAfterFinished is the number of seconds to retain the search after it has finished.
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty" protobuf:"varint,2,opt,name=ttlSecondsAfterFinished"`
}

// SearchStatus defines the observed state of Search.
type SearchStatus struct {
	// Conditions represent the latest available observations of a Pipeline's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Pipeline's current state."`

	// SearchJob is a reference to the job associated with this search.
	// A nil value indicates that the scan job has not been created yet.
	// +optional
	SearchJob *v1.ObjectReference `json:"scanJob,omitempty" description:"A reference to the scan job associated with this pipeline."`

	// Failed indicates if the search has failed, meaning that the crawler exited with a non-zero status code.
	// A nil value indicates that the search is still in progress or has not started yet.
	// +optional
	Failed *bool `json:"failed,omitempty" description:"Indicates if the pipeline has failed."`

	// StartTime is the time when the search started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" description:"The time when the search was started, nil represents that the search has not started yet."`

	// CompletionTime is the time when the search completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" description:"The time when the search completed, nil represents that the search has not completed yet."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Search is the Schema for the searches API
type Search struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Search
	// +required
	Spec SearchSpec `json:"spec"`

	// status defines the observed state of Search
	// +optional
	Status SearchStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// SearchList contains a list of Search
type SearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Search `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Search{}, &SearchList{})
}
