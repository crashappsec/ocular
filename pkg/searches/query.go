// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package searches

import (
	"context"
	"fmt"
	"strings"

	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

type ListOption = runtime.ListOption

func WithCrawler(name string) ListOption {
	return func(meta metav1.ListOptions) metav1.ListOptions {
		meta.LabelSelector = fmt.Sprintf("%s=%s", labelCrawlerName, name)
		return meta
	}
}

func List(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	opts ...ListOption,
) ([]schemas.Search, error) {
	jobs, err := runtime.ListJobs(ctx, jobInterface, searchJobType, opts...)
	if err != nil {
		return nil, err
	}

	var (
		searches []schemas.Search
		merr     *multierror.Error
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
		s, sErr := searchFromScanJob(ctx, id, &job)
		if sErr != nil {
			merr = multierror.Append(
				merr,
				fmt.Errorf("failed to generate search from job %s: %w", job.Name, sErr),
			)
			continue
		}
		searches = append(searches, s)
	}
	zap.L().Debug("found searches", zap.Int("count", len(searches)))
	return searches, merr.ErrorOrNil()
}

func Get(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id schemas.ExecutionID,
) (schemas.Search, error) {
	searchJob, err := runtime.GetJob(ctx, jobInterface, id, searchJobType)
	if err != nil {
		return schemas.Search{}, fmt.Errorf("unable to get search job for search %s: %w", id, err)
	}
	return searchFromScanJob(ctx, id, searchJob)
}

func searchFromScanJob(
	_ context.Context,
	id schemas.ExecutionID,
	searchJob *batchV1.Job,
) (schemas.Search, error) {
	search := schemas.Search{
		ID:     id,
		Status: runtime.RunStatusFromJobStatus(searchJob.Status),
	}

	parameters := make(map[string]string)
	if len(searchJob.Spec.Template.Spec.Containers) != 0 {
		anyContainer := searchJob.Spec.Template.Spec.Containers[0]
		if len(anyContainer.Env) != 0 {
			for _, env := range anyContainer.Env {
				if strings.HasPrefix(env.Name, schemas.ParamEnvVarPrefix) {
					paramName := strings.TrimPrefix(env.Name, schemas.ParamEnvVarPrefix)
					parameters[paramName] = env.Value
				}
			}
		}
	}
	search.Parameters = parameters

	if len(searchJob.Annotations) != 0 {
		if crawlerName, ok := searchJob.Annotations[annotationCrawlerName]; ok {
			search.CrawlerName = crawlerName
		}
	}

	return search, nil
}
