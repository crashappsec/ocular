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
	CrawlerRef v1.ObjectReference `json:"crawlerRef,omitempty" protobuf:"bytes,1,opt,name=crawlerRef"`

	// Parameters is a list of parameters to pass to the crawler as environment variables.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Parameters []ParameterSetting `json:"parameters,omitempty" protobuf:"bytes,2,rep,name=parameters"`

	// TTLSecondsAfterFinished is the number of seconds to retain the search after it has finished.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty" protobuf:"varint,2,opt,name=ttlSecondsAfterFinished"`
}

// SearchStatus defines the observed state of Search.
type SearchStatus struct {
	// Conditions latest available observations of an object's current state. When a Search
	// fails, one of the conditions will have type [FailedConditionType] and status true.
	// A search is considered finished when it is in a terminal condition, either
	// [CompleteConditionType] or [FailedConditionType]. A Search cannot have both the [CompleteConditionType]  and FailedConditionType] conditions.
	//
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// StartTime is the time when the search started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" description:"The time when the search was started, nil represents that the search has not started yet."`

	// CompletionTime is the time when the search completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" description:"The time when the search completed, nil represents that the search has not completed yet."`

	// CronSearchControllerName is the name of the controller that created this search.
	// +optional
	CronSearchControllerName *string `json:"cronSearchControllerName,omitempty" description:"The name of the controller that created this search."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:selectablefield:JSONPath=`.status.cronSearchControllerName`
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
