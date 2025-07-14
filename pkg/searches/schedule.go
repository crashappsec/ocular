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
	"regexp"
	"strings"

	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

// https://stackoverflow.com/questions/14203122/create-a-regular-expression-for-cron-statement
var cronScheduleRegex = regexp.MustCompile(
	`^(@(annually|yearly|monthly|weekly|daily|hourly|reboot))|(@every (\d+(ns|us|µs|ms|s|m|h))+)|((((\d+,)+\d+|(\d+(\/|-)\d+)|\d+|\*) ?){5,7})$`,
)

func SetSchedule(
	ctx context.Context,
	jobInterface typedBatchV1.CronJobInterface,
	ctxName string, crawlerName string, crawler resources.Crawler,
	schedule string, params map[string]string,
) (schemas.ScheduledSearch, error) {
	id := uuid.New()
	l := zap.L().With(
		zap.String("id", id.String()),
		zap.String("source", "Run-pipeline"),
		zap.Any("crawler", crawlerName),
		zap.Any("ctx_name", ctxName),
		zap.Any("params", params),
	)
	l.Info("setting crawler schedule")

	if !cronScheduleRegex.MatchString(schedule) {
		l.Error("invalid cron schedule", zap.String("schedule", schedule))
		return schemas.ScheduledSearch{}, errs.New(
			errs.TypeBadRequest,
			nil,
			"invalid cron schedule",
		)
	}
	jobReq := buildJob(id, ctxName, crawlerName, crawler, params)

	cronJob := &batchV1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crawler-" + id.String(),
			Labels: runtime.CreateLabels(map[string]string{
				"resource":       "crawler-schedule",
				"crawler-run-id": id.String(),
				labelCrawlerName: crawlerName,
			}),
			Annotations: map[string]string{
				annotationCrawlerName: crawlerName,
				annotationRunID:       id.String(),
			},
		},
		Spec: batchV1.CronJobSpec{
			Schedule: schedule,
			JobTemplate: batchV1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"resource": "crawler-scheduled-run",
					},
				},
				Spec: jobReq.Spec,
			},
		},
	}

	_, err := jobInterface.Create(ctx, cronJob, metav1.CreateOptions{})
	if err != nil {
		l.Error("failed to create cron job", zap.Error(err))
		return schemas.ScheduledSearch{}, err
	}
	l.Info("scheduled crawler run", zap.String("schedule", schedule))
	return schemas.ScheduledSearch{
		ID:          id,
		Parameters:  params,
		Schedule:    schedule,
		CrawlerName: crawlerName,
	}, nil
}

func RemoveSchedule(
	ctx context.Context,
	jobInterface typedBatchV1.CronJobInterface,
	id RunID,
) error {
	policy := metav1.DeletePropagationBackground
	err := jobInterface.Delete(ctx, "crawler-"+id.String(), metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return errs.New(errs.TypeNotFound, err, "pipeline not found")
		}
		if errors.IsForbidden(err) {
			return errs.New(errs.TypeForbidden, err, "unauthorized to access pipeline")
		}
		return errs.New(errs.TypeUnknown, err, "unable to delete job %s", id)
	}
	return nil
}

func ListSchedules(
	ctx context.Context,
	jobInterface typedBatchV1.CronJobInterface,
	opts ...ListOption,
) ([]schemas.ScheduledSearch, error) {
	listOpts := metav1.ListOptions{}
	for _, opt := range opts {
		listOpts = opt(listOpts)
	}

	list, err := jobInterface.List(ctx, listOpts)
	l := zap.L().With(zap.String("source", "ListSchedules"))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errs.New(errs.TypeNotFound, err, "pipeline not found")
		}
		if errors.IsForbidden(err) {
			return nil, errs.New(errs.TypeForbidden, err, "unauthorized to access pipeline")
		}
		return nil, errs.New(errs.TypeUnknown, err, "unable to list schedules")
	}

	var schedules []schemas.ScheduledSearch
	for _, job := range list.Items {
		runID, err := ParseRunID(job.Annotations[annotationRunID])
		if err != nil {
			l.Error("error parsing run ID", zap.Error(err))
			continue
		}

		params := make(map[string]string)
		if len(job.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
			env := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
			for _, e := range env {
				if strings.HasPrefix(e.Name, schemas.ParamEnvVarPrefix) {
					params[strings.TrimPrefix(e.Name, schemas.ParamEnvVarPrefix)] = e.Value
				}
			}
		}

		schedules = append(schedules, schemas.ScheduledSearch{
			ID:         runID,
			Schedule:   job.Spec.Schedule,
			Parameters: params,
		})
	}

	return schedules, nil
}
