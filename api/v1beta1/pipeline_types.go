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

type PipelineSpec struct {
	// DownloaderRef is a reference to the downloader that will be used in this pipeline.
	// It should point to a valid Downloader resource in the same namespace.
	// +required
	DownloaderRef v1.ObjectReference `json:"downloaderRef" protobuf:"bytes,1,opt,name=downloaderRef"`

	// ProfileRef is a reference to the profile that will be used in this pipeline.
	// It should point to a valid Profile resource in the same namespace.
	// +required
	ProfileRef v1.ObjectReference `json:"profileRef" protobuf:"bytes,2,opt,name=profileRef"`

	// Target is the actual software asset that will be processed by this pipeline.
	// It is up to the Downloader to interpret the target correctly.
	// +required
	Target Target `json:"target" protobuf:"bytes,3,opt,name=target"`

	// ScanServiceAccountName is the name of the service account that will be used to run the scan job.
	// If not set, the default service account of the namespace will be used.
	// +optional
	ScanServiceAccountName string `json:"scanServiceAccountName,omitempty" protobuf:"bytes,4,opt,name=scanServiceAccountName" description:"The name of the service account that will be used to run the scan job."`

	// UploadServiceAccountName is the name of the service account that will be used to run the upload job.
	// If not set, the default service account of the namespace will be used.
	// +optional
	UploadServiceAccountName string `json:"uploadServiceAccountName,omitempty" protobuf:"bytes,5,opt,name=uploadServiceAccountName" description:"The name of the service account that will be used to run the upload job."`

	// TTLSecondsAfterFinished
	// If set, the pipeline and its associated resources will be automatically deleted
	// after the specified number of seconds have passed since the pipeline finished.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"  protobuf:"bytes,6,opt,name=ttlSecondsAfterFinished"`
}

type PipelineStatus struct {
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

	// ScanJobOnly indicates if the pipeline is configured to run only the scan job without uploading results.
	// This is true when the profile associated with the pipeline has no artifacts or uploaders defined.
	// +optional
	ScanJobOnly bool `json:"scanJobOnly,omitempty" description:"Indicates if the pipeline is configured to run only the scan job without uploading results."`

	// StartTime is the time when the pipeline started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" description:"The time when the pipeline was started, nil represents that the pipeline has not started yet."`

	// CompletionTime is the time when the pipeline completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" description:"The time when the pipeline completed, nil represents that the pipeline has not completed yet."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Pipeline is the Schema for the downloaders API
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Pipeline
	// +required
	Spec PipelineSpec `json:"spec"`

	// status defines the observed state of Pipeline
	// +optional
	Status PipelineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pipeline{}, &PipelineList{})
}
