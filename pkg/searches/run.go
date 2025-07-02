// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package searches provides functions to manage invocation of a [Search].
// A search is a long-running process that is the invocation of a [crawlers.Crawler].
// It will have the ability to trigger a [github.com/crashappsec/ocular/pkg/pipelines.Pipeline] or another [Search].
package searches

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/internal/config"
	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/identities"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

const (
	labelCrawlerName      = "crawler-name"
	searchJobType         = "search"
	annotationCrawlerName = "crashoverride.run/crawler-name"
	annotationRunID       = "crashoverride.run/search-id"

	secretVolumeName = "search-secret-volume"
)

type RunID = uuid.UUID

func ParseRunID(s string) (RunID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return RunID{}, errs.New(errs.TypeBadRequest, nil, "invalid UUID")
	}
	return RunID(id), nil
}

func Run(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	ctxName, crawlerName string,
	crawler resources.Crawler,
	params map[string]string,
) (schemas.Search, error) {
	id := uuid.New()
	l := zap.L().With(
		zap.String("id", id.String()),
		zap.String("source", "run-search"),
		zap.Any("crawler", crawlerName),
		zap.Any("ctx_name", ctxName),
		zap.Any("params", params),
	)
	l.Info("trigger crawler")

	jobReq := buildJob(id, ctxName, crawlerName, crawler, params)
	_, err := jobInterface.Create(ctx, jobReq, metav1.CreateOptions{})
	if err != nil {
		return schemas.Search{}, err
	}

	l.Info("begun crawler run")

	return schemas.Search{
		ID:          id,
		Parameters:  params,
		CrawlerName: crawlerName,
		Status:      schemas.RunStatusPending,
	}, nil
}

func buildJob(
	id RunID,
	ctxName string,
	crawlerName string,
	crawler resources.Crawler,
	params map[string]string,
) *batchV1.Job {
	l := zap.L().With(
		zap.String("crawler", crawlerName),
		zap.String("ctx_name", ctxName),
		zap.Any("params", params),
		zap.String("id", id.String()),
	)
	scheme := "http"
	if config.State.API.TLS.Enabled {
		scheme = "https"
	}

	tokenPath, tokenVolume, tokenVolumeMount := identities.CreateTokenVolume(
		identities.TokenAudienceCrawler,
	)

	globalEnvVars := []v1.EnvVar{
		{
			Name:  schemas.EnvVarOcularTokenPath,
			Value: tokenPath,
		},
		{
			Name:  schemas.EnvVarCrawlerName,
			Value: crawlerName,
		},
		{
			Name:  schemas.EnvVarContextName,
			Value: ctxName,
		},
		{
			Name:  schemas.EnvVarAPIBaseURL,
			Value: fmt.Sprintf("%s://%s:%d", scheme, config.State.API.Host, config.State.API.Port),
		},
	}

	for name, value := range params {
		_, exists := crawler.Parameters[schemas.FormatParamName(name)]
		if !exists {
			l.Warn("parameter not found", zap.String("param", name))
			continue
		}
		globalEnvVars = append(globalEnvVars, v1.EnvVar{
			Name:  schemas.ParameterNameToEnv(name),
			Value: value,
		})
	}

	secretVolume := v1.Volume{
		Name: secretVolumeName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: config.State.Secrets.SecretName,
			},
		},
	}

	volumes := []v1.Volume{
		secretVolume,
		tokenVolume,
	}

	containers := []v1.Container{
		runtime.CreateContainer(
			fmt.Sprintf("crawler-%s", crawlerName),
			crawler.UserContainer,
			secretVolumeName,
			runtime.ContainerVolumesOpt(tokenVolumeMount),
			runtime.ContainerEnvOpt(globalEnvVars...),
		),
	}

	jobOpts := []runtime.JobOpt{
		runtime.JobOptWithLabels(map[string]string{
			labelCrawlerName: crawlerName,
		}),
		runtime.JobOptWithAnnotations(map[string]string{
			annotationCrawlerName: crawlerName,
			annotationRunID:       id.String(),
		}),
		runtime.JobOptWithVolumes(volumes),
	}

	if config.State.Runtime.CrawlersServiceAccount != "" {
		jobOpts = append(
			jobOpts,
			runtime.JobOptWithServiceAccountName(config.State.Runtime.CrawlersServiceAccount),
		)
	}

	job := runtime.BuildJob(
		searchJobType,
		id,
		nil,
		containers,
		jobOpts...,
	)

	return &job
}
