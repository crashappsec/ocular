// Copyright (C) 2025-2026 Crash Override, Inc.
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

const (
	// PipelineResultsDirectory is the directory where the target scan results will be stored.
	// This directory should contain all the [ProfileSpec.Artifacts] after the scan is complete.
	PipelineResultsDirectory = "/mnt/results"

	// PipelineMetadataDirectory is the directory where the target metadata will be stored.
	// This directory should contain all the [DownloaderSpec.MetadataFiles] after the download is complete.
	PipelineMetadataDirectory = "/mnt/metadata"

	// PipelineTargetDirectory is the directory where the pipeline target will be stored.
	// This directory is where the [Downloader] should write the target to be scanned to.
	PipelineTargetDirectory = "/mnt/target"

	// PipelineLabelKey is the label key used to identify resources associated with a specific pipeline.
	// It will contain the name of the pipeline as its value.
	PipelineLabelKey = Group + "/pipeline"

	// ProfileLabelKey is the label key used to identify pipelines created from a specific profile.
	ProfileLabelKey = Group + "/profile"
	// DownloaderLabelKey is the label key used to identify pipelines created from a specific downloader.
	DownloaderLabelKey = Group + "/downloader"
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

	// TTLSecondsMaxLifetime
	// If set, the pipeline and its associated resources will be automatically deleted
	// after the specified number of seconds have passed since the pipeline was created,
	// regardless of its state.
	// +optional
	TTLSecondsMaxLifetime *int32 `json:"ttlSecondsMaxLifetime,omitempty" protobuf:"bytes,7,opt,name=TTLSecondsMaxLifetime" description:"If set, the pipeline and its associated resources will be automatically deleted after the specified number of seconds have passed since the pipeline was created, regardless of its state."`
}

// PipelinePhase is a label for the condition of a pipeline at the current time.
// +enum
type PipelinePhase string

// These are the valid statuses of pods.
const (
	// PipelinePending means the pipeline pods are still creating and have not been accepted by the system,
	// but one or more of the containers has not been started.
	PipelinePending PipelinePhase = "Pending"
	// PipelineDownloading means the pipeline scan pod is in the process of downloading the target.
	PipelineDownloading PipelinePhase = "Downloading"
	// PipelineScanning means that the pipeline scan pod is in the process of scanning the target.
	PipelineScanning PipelinePhase = "Scanning"
	// PipelineUploading means that the pipeline upload pod is in the process of uploading the results.
	PipelineUploading PipelinePhase = "Uploading"
	// PipelineSucceeded means that all containers in the pipeline have terminated in success
	// (exited with a zero exit code).
	PipelineSucceeded PipelinePhase = "Succeeded"
	// PipelineFailed means that one or more containers in the pipeline
	// (downloader, uploader, scanner) have terminated in a failure
	// (exited with a non-zero exit code or was stopped by the system).
	// View the pipeline's Reason or Conditions for more details.
	PipelineFailed PipelinePhase = "Failed"

	// PipelineStateUnknown means that for some reason the state of the pod could not be obtained, typically due
	// to an error in communicating with the host of the pod.
	PipelineStateUnknown PipelinePhase = "Unknown"
)

const (

	// PipelineScanPodCreatedConditionType is the condition type used when the scan pod for a pipeline has been created.
	// If this condition is true, it indicates that the scan pod has been successfully created.
	// If this condition is false, it indicates that there was an error creating the scan pod.
	// The absence of this condition indicates that the scan pod has not been created yet.
	PipelineScanPodCreatedConditionType = "PipelineScanPodCreated"

	// PipelineUploadPodCreatedConditionType is the condition type used when the upload pod for a pipeline has been created.
	// If this condition is true, it indicates that the upload pod has been successfully created.
	// If this condition is false, it indicates that there was an error creating the upload pod.
	// The absence of this condition indicates that the upload pod has not been created yet (or won't be
	// created if the pipeline is scanPodOnly).
	PipelineUploadPodCreatedConditionType = "PipelineUploadPodCreated"

	// PipelineCompletedSuccessfullyConditionType is the condition type used when a pipeline has completed successfully.
	// If this condition is true, it indicates that the pipeline has completed all its stages without errors.
	// If this condition is false, it indicates that the pipeline has completed, but with a failure.
	// The absence of this condition indicates that the pipeline is still in progress.
	PipelineCompletedSuccessfullyConditionType = "PipelineCompletedSuccessful"
)

// PipelineStageStatus represents the status of a specific (downloader, uploader, scanners)
// stage in the pipeline.
// +enum
type PipelineStageStatus string

const (
	// PipelineStageNotStarted indicates that the stage has not started yet.
	PipelineStageNotStarted PipelineStageStatus = "NotStarted"
	// PipelineStageInProgress indicates that the stage is currently in progress.
	PipelineStageInProgress PipelineStageStatus = "InProgress"
	// PipelineStageCompleted indicates that the stage has completed successfully.
	PipelineStageCompleted PipelineStageStatus = "Completed"
	// PipelineStageFailed indicates that the stage has failed.
	PipelineStageFailed PipelineStageStatus = "Failed"
	// PipelineStageSkipped indicates that the stage was skipped.
	// either because the pipeline is configured to skip it,
	// or due to an earlier failure in the pipeline.
	PipelineStageSkipped PipelineStageStatus = "Skipped"
)

// PipelineStageStatuses represents the status of each stage in the pipeline.
type PipelineStageStatuses struct {
	// DownloadStatus represents the current status of the download stage.
	// +optional
	DownloadStatus PipelineStageStatus `json:"downloadStatus" description:"The current status of the download stage."`

	// ScanStatus represents the current status of the scan stage.
	// +optional
	ScanStatus PipelineStageStatus `json:"scanStatus" description:"The current status of the scan stage."`

	// UploadStatus represents the current status of the upload stage.
	// +optional
	UploadStatus PipelineStageStatus `json:"uploadStatus" description:"The current status of the upload stage."`
}

type PipelineStatus struct {
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

	// ScanPodOnly indicates if the pipeline is configured to run only the scan job without uploading results.
	// This is true when the profile associated with the pipeline has no artifacts or uploaders defined.
	// +optional
	ScanPodOnly bool `json:"scanPodOnly,omitempty" description:"Indicates if the pipeline is configured to run only the scan job without uploading results."`

	// StartTime is the time when the pipeline started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" description:"The time when the pipeline was started, nil represents that the pipeline has not started yet."`

	// CompletionTime is the time when the pipeline completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" description:"The time when the pipeline completed, nil represents that the pipeline has not completed yet."`

	// PipelinePhase is the current phase of the pipeline.
	// For more information about a particular stage in the pipeline, refer to StageStatuses.
	// +optional
	Phase PipelinePhase `json:"phase" description:"The current state of the pipeline."`

	// StageStatuses represents the current status of each stage in the pipeline.
	// +optional
	StageStatuses PipelineStageStatuses `json:"stageStatuses,omitempty,omitzero" description:"The current status of each stage in the pipeline."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// Pipeline is the Schema for the downloaders API
type Pipeline struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Pipeline
	// +required
	Spec PipelineSpec `json:"spec"`

	// status defines the observed state of Pipeline
	// +optional
	Status PipelineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PipelineList contains a list of Pipeline
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Pipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pipeline{}, &PipelineList{})
}
