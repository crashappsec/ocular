// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CronSearchSpec defines the desired state of CronSearch
type CronSearchSpec struct {
	SearchSpec `json:",inline"`

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	Schedule string `json:"schedule" protobuf:"bytes,1,opt,name=schedule"`
}

// CronSearchStatus defines the observed state of CronSearch.
type CronSearchStatus struct {
	// CronSearchJob is a reference to the CronJob associated with this CronSearch.
	// A nil value indicates that the CronJob has not been created yet.
	// +optional
	CronSearchJob *corev1.ObjectReference `json:"cronSearchJob,omitempty" protobuf:"bytes,1,opt,name=cronSearchJob"`

	// CronSearchJobStatus represents the status of the CronJob associated with this CronSearch.
	// A nil value indicates that the CronJob has not been created yet.
	// This is useful to check status of the CronJob without having to fetch the CronJob resource itself.
	// +optional
	CronSearchJobStatus *batchv1.CronJobStatus `json:"cronSearchJobStatus,omitempty" protobuf:"bytes,2,opt,name=cronSearchJobStatus"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CronSearch is the Schema for the cronsearches API
type CronSearch struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of CronSearch
	// +required
	Spec CronSearchSpec `json:"spec"`

	// status defines the observed state of CronSearch
	// +optional
	Status CronSearchStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// CronSearchList contains a list of CronSearch
type CronSearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronSearch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronSearch{}, &CronSearchList{})
}
