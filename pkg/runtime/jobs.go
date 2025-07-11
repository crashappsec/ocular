// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import (
	"context"
	"time"

	"github.com/crashappsec/ocular/internal/config"
	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/utils/ptr"
)

// JobOpt is a function that modifies a Job object.
type JobOpt = func(job batchV1.Job) batchV1.Job

// JobOptWithLabels adds labels to the Job object.
func JobOptWithLabels(labels map[string]string) JobOpt {
	return func(job batchV1.Job) batchV1.Job {
		if job.Labels == nil {
			job.Labels = make(map[string]string)
		}
		for k, v := range labels {
			job.Labels[k] = v
		}
		return job
	}
}

// JobOptWithAnnotations adds annotations to the Job object.
func JobOptWithAnnotations(annotations map[string]string) JobOpt {
	return func(job batchV1.Job) batchV1.Job {
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			job.Annotations[k] = v
		}
		return job
	}
}

// JobOptWithServiceAccountName sets the service account name for the Job object.
// This is useful for setting the service account that the job should run as.
func JobOptWithServiceAccountName(name string) JobOpt {
	return func(job batchV1.Job) batchV1.Job {
		job.Spec.Template.Spec.ServiceAccountName = name
		return job
	}
}

// JobOptWithVolumes adds volumes to the Job object.
func JobOptWithVolumes(volumes []v1.Volume) JobOpt {
	return func(job batchV1.Job) batchV1.Job {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, volumes...)
		return job
	}
}

// JobOptWithTimeout sets the timeout for the Job object.
func JobOptWithTimeout(duration time.Duration) JobOpt {
	return func(job batchV1.Job) batchV1.Job {
		job.Spec.ActiveDeadlineSeconds = ptr.To[int64](int64(duration.Seconds()))
		return job
	}
}

// BuildJob creates a new Job object with the given parameters.
// It takes a resource name, an ID, and a list of containers.
// It also accepts a variadic number of JobOpt options to modify the Job object.
func BuildJob(
	resourceName string,
	id schemas.ExecutionID,
	initContainers, containers []v1.Container,
	opts ...JobOpt,
) batchV1.Job {
	var imageSecrets []v1.LocalObjectReference
	for _, v := range config.State.Runtime.ImagePullSecrets {
		imageSecrets = append(imageSecrets, v1.LocalObjectReference{
			Name: v,
		})
	}

	jobTTL := config.State.Runtime.JobTTL.Seconds()

	jobReq := batchV1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName + "-" + id.String(),
			Labels: CreateLabels(map[string]string{
				LabelResource: resourceName,
				LabelID:       id.String(),
			}),
		},
		Spec: batchV1.JobSpec{
			TTLSecondsAfterFinished: ptr.To[int32](int32(jobTTL)), // 3 minutes
			BackoffLimit:            ptr.To[int32](0),
			Parallelism:             ptr.To[int32](1),
			Completions:             ptr.To[int32](1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: CreateLabels(map[string]string{
						LabelResource: resourceName + "-execution",
						LabelID:       id.String(),
					}),
				},
				Spec: v1.PodSpec{
					RestartPolicy:    "Never",
					InitContainers:   initContainers,
					Containers:       containers,
					ImagePullSecrets: imageSecrets,
				},
			},
		},
	}

	for _, opt := range opts {
		jobReq = opt(jobReq)
	}
	return jobReq
}

// StopJob stops a job with the given ID.
// It uses the provided [typedBatchV1.JobInterface] to delete the job.
// It returns an error if the job cannot be deleted or if it is not found.
func StopJob(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id schemas.ExecutionID,
	resourceType string,
) error {
	policy := metav1.DeletePropagationBackground
	err := jobInterface.Delete(
		ctx,
		resourceType+"-"+id.String(),
		metav1.DeleteOptions{
			PropagationPolicy: &policy,
		},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return errs.New(errs.TypeNotFound, err, "pipeline not found")
		}
		if errors.IsForbidden(err) {
			return errs.New(
				errs.TypeForbidden,
				err,
				"api service account is unauthorized to access pipeline",
			)
		}
		return errs.New(errs.TypeUnknown, err, "unable to delete job %s", id)
	}
	return nil
}

func GetJob(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id schemas.ExecutionID,
	resourceType string,
) (*batchV1.Job, error) {
	job, err := jobInterface.Get(ctx, resourceType+"-"+id.String(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errs.New(errs.TypeNotFound, err, "pipeline not found")
		}
		if errors.IsForbidden(err) {
			return nil, errs.New(
				errs.TypeForbidden,
				err,
				"api service account is unauthorized to access pipeline",
			)
		}
		return nil, errs.New(errs.TypeUnknown, err, "unable to get job %s", id)
	}
	return job, nil
}

// RunStatusFromJobStatus converts a Kubernetes JobStatus to a [schema.RunStatus].
// It will return a [schemas.RunStatus] based on the JobStatus.
// [schemas.RunStatusUnknown] is returned if the status cannot be determined.
// [schemas.RunStatusError] is returned if the job has failed, and the reason is not "Deleted".
// [schemas.RunStatusCancelled] is returned if the job has failed and the reason is "Deleted".
// [schemas.RunStatusNotRan] is never returned, as it is not applicable to JobStatus.
func RunStatusFromJobStatus(s batchV1.JobStatus) schemas.RunStatus {
	zap.L().Debug("listing job status", zap.Any("status", s))
	status := schemas.RunStatusUnknown
	switch {
	case s.Active > 0:
		status = schemas.RunStatusRunning
	case s.Failed > 0:
		for _, condition := range s.Conditions {
			if condition.Type == batchV1.JobFailed {
				if condition.Reason == "Deleted" {
					status = schemas.RunStatusCancelled
				} else {
					status = schemas.RunStatusError
				}
				break
			}
		}
	case s.Succeeded > 0:
		status = schemas.RunStatusSuccess
	}
	return status
}

type ListOption = func(meta metav1.ListOptions) metav1.ListOptions

func ListJobs(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	resourceType string,
	opts ...ListOption,
) ([]batchV1.Job, error) {
	lOpts := metav1.ListOptions{
		LabelSelector: LabelResource + "=" + resourceType,
	}

	for _, opt := range opts {
		lOpts = opt(lOpts)
	}
	jobs, err := jobInterface.List(ctx, lOpts)
	if err != nil {
		if errors.IsForbidden(err) {
			return nil, errs.New(
				errs.TypeForbidden,
				err,
				"api service account is unauthorized to list jobs",
			)
		}
		return nil, errs.New(
			errs.TypeUnknown,
			err,
			"unable to list jobs for resource %s",
			resourceType,
		)
	}
	return jobs.Items, nil
}

func IDFromJob(job *batchV1.Job) (schemas.ExecutionID, error) {
	if job == nil {
		return schemas.ExecutionID{}, errs.New(errs.TypeBadRequest, nil, "job is nil")
	}

	idStr, ok := job.Labels[LabelID]
	if !ok {
		return schemas.ExecutionID{}, errs.New(
			errs.TypeBadRequest,
			nil,
			"job does not have an ID label",
		)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return schemas.ExecutionID{}, errs.New(
			errs.TypeBadRequest,
			err,
			"invalid job ID: %s",
			idStr,
		)
	}

	return schemas.ExecutionID(id), nil
}
