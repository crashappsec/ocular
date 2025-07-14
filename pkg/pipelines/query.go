// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package pipelines

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

func Get(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id schemas.ExecutionID,
) (Pipeline, error) {
	pipeline := Pipeline{
		ID:      id,
		Profile: "unknown",
	}
	scanJob, err := runtime.GetJob(ctx, jobInterface, id, scanJobType)
	if err != nil {
		return pipeline, fmt.Errorf("unable to get scan job for pipeline %s: %w", id, err)
	}
	return pipelineFromScanJob(ctx, jobInterface, id, scanJob)
}

func pipelineFromScanJob(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id uuid.UUID,
	scanJob *batchV1.Job,
) (Pipeline, error) {
	pipeline := Pipeline{
		ID: id,
	}
	pipeline.ScanStatus = runtime.RunStatusFromJobStatus(scanJob.Status)
	if pipeline.ScanStatus != schemas.RunStatusSuccess &&
		pipeline.ScanStatus != schemas.RunStatusPending &&
		pipeline.ScanStatus != schemas.RunStatusRunning {
		pipeline.UploadStatus = schemas.RunStatusNotRan
	} else {
		uploadJob, err := runtime.GetJob(ctx, jobInterface, id, uploadJobType)
		if err != nil {
			if errors.IsNotFound(err) {
				pipeline.UploadStatus = schemas.RunStatusNotRan
			} else {
				return pipeline, fmt.Errorf("unable to get upload job for pipeline %s: %w", id, err)
			}
		} else {
			uploadStatus := runtime.RunStatusFromJobStatus(uploadJob.Status)
			if (pipeline.ScanStatus == schemas.RunStatusRunning || pipeline.ScanStatus == schemas.RunStatusPending) && uploadStatus == schemas.RunStatusRunning {
				pipeline.UploadStatus = schemas.RunStatusPending
			} else {
				pipeline.UploadStatus = uploadStatus
			}
		}
	}

	pipeline.Target = schemas.Target{
		Downloader: scanJob.Annotations[annotationTargetDownloader],
		Identifier: scanJob.Annotations[annotationTargetIdentifier],
		Version:    scanJob.Annotations[annotationTargetVersion],
	}

	pipeline.Profile = scanJob.Annotations[annotationProfileName]

	return pipeline, nil
}

func List(ctx context.Context, jobInterface typedBatchV1.JobInterface) ([]Pipeline, error) {
	jobs, err := runtime.ListJobs(ctx, jobInterface, scanJobType)
	if err != nil {
		return nil, err
	}

	var (
		pipelines []Pipeline
		merr      *multierror.Error
	)
	for _, job := range jobs {
		id, idErr := runtime.IDFromJob(&job)
		if idErr != nil {
			merr = multierror.Append(
				merr,
				fmt.Errorf("failed to get ID from job %s: %w", job.Name, idErr),
			)
			continue
		}
		p, pErr := pipelineFromScanJob(ctx, jobInterface, id, &job)
		if pErr != nil {
			merr = multierror.Append(
				merr,
				fmt.Errorf("failed to create pipeline from job %s: %w", job.Name, pErr),
			)
			continue
		}
		pipelines = append(pipelines, p)
	}
	zap.L().Debug("found pipelines", zap.Int("count", len(pipelines)))
	return pipelines, merr.ErrorOrNil()
}
