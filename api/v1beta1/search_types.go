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

const (
	// SearchLabelKey is the label key used to identify resources associated with a specific search.
	SearchLabelKey = Group + "/search"

	// CrawlerLabelKey is the label key used to identify searches created from a specific crawler.
	CrawlerLabelKey = Group + "/crawler"

	// PipelineTemplateAnnoation is the annotation containing the JSON
	// encoded pipeline template for the search scheduler.
	PipelineTemplateAnnotation = Group + "/pipeineTemplate.json"
)

// SearchSpec defines the desired state of Search
type SearchSpec struct {
	// CrawlerRef is a reference to the crawler that will be run in this search.
	// It should point to a valid Crawler resource in the same namespace.
	// +required
	CrawlerRef ParameterizedObjectReference `json:"crawlerRef,omitempty" protobuf:"bytes,1,opt,name=crawlerRef"`

	// TTLSecondsAfterFinished is the number of seconds to retain the search after it has finished.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty" protobuf:"varint,2,opt,name=ttlSecondsAfterFinished"`

	// ServiceAccountNameOverride is the name of the service account that will be used to run the scan job.
	// If not set, the default service account of the namespace will be used.
	// If not specified, a temporary ServiceAccount will be created for the search.
	// NOTE: This ServiceAccount must exist in the same namespace as the Search.
	// +optional
	ServiceAccountNameOverride string `json:"serviceAccountNameOverride,omitempty" protobuf:"bytes,4,opt,name=serviceAccountNameOverride" description:"The name of the service account that will be used to run the scan job."`

	// Scheduler represents the configuration of the scheduler sidecar
	// +optional
	Scheduler SearchSchedulerSpec `json:"scheduler,omitempty"`
}

// SearchSchedulerSpec configures the scheduler sidecar container
// present on the search pod. The sechduler will read a [Target] JSON
// spec from the unix socket [EnvVarPipelineFifo] and create pipelines
// for each target. The pipeline will be derived from the [PipelineTemplate]
// (with the target replaced) and
type SearchSchedulerSpec struct {
	// PipelineTemplate is the template for pipelines that will be created from this search.
	// The pipeline template will be read by the ocular sidecar container, and when it receives
	// targets via the unix socket [EnvVarPipelineSocket], it will create a pipeline from the template
	// with the received target set. If omitted, sidecar container will be disabled
	// +optional
	PipelineTemplate PipelineTemplate `json:"pipelineTemplate,omitempty"`

	// IntervalSeconds represents the amount of time to wait
	// between creating pipelines. If not set, scheduler defaults to
	// 60 (1 minute).
	// +optional
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`
}

// PipelineTemplate is the template for pipelines
// that are created from a Search
type PipelineTemplate struct {
	// Standard object's metadata of the jobs created from this template.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// Since this is a template, only generateName, labels and annotations will be used.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// Spec is the template for created pipelines.
	// The "target" field will be overrwritten
	// +optional
	Spec PipelineSpec `json:"spec,omitempty"`
}

// SearchStatus defines the observed state of Search.
type SearchStatus struct {
	// Conditions latest available observations of an object's current state. When a Search
	// fails, one of the conditions will have type [FailedConditionType] and status true.
	// A search is considered finished when it is in a terminal condition, either
	// [CompleteConditionType] or [FailedConditionType]. A Search cannot have both the [CompleteConditionType]  and FailedConditionType] conditions.
	//
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/
	// +listType=map
	// +listMapKey=type
	// +optional
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
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Search
	// +required
	Spec SearchSpec `json:"spec"`

	// status defines the observed state of Search
	// +optional
	Status SearchStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SearchList contains a list of Search
type SearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Search `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Search{}, &SearchList{})
}
