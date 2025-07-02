// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package pipelines implements the logic to run a pipeline
// for a target using a profile. It will accept a target to scan,
// the name of the downloader that should pull the target down, and a profile. It will
// then create a job to run the downloader first, then all scanners in the given profile.
// After it will start a job with the uploaders from the profile, with the resulting artifacts
// from the scanners.
package pipelines

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"go.uber.org/zap"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedV1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Run(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	serviceInterface typedV1.ServiceInterface,
	svcNamespace string,
	target schemas.Target,
	dl resources.Downloader,
	profileName string,
	profile resources.Profile,
	uploaderStorage resources.UploaderStorageBackend,
) (Pipeline, error) {
	id := uuid.New()
	l := zap.L().With(
		zap.String("id", id.String()),
		zap.String("source", "create-pipeline"),
		zap.Any("target", target),
		zap.Any("profile-name", profileName),
	)
	scanStatus, uploadStatus := schemas.RunStatusPending, schemas.RunStatusPending
	l.Info("running pipeline")

	globalEnvVars := []v1.EnvVar{
		{
			Name:  schemas.EnvVarTargetDownloader,
			Value: target.Downloader,
		},
		{
			Name:  schemas.EnvVarTargetIdentifier,
			Value: target.Identifier,
		},
		{
			Name:  schemas.EnvVarTargetVersion,
			Value: target.Version,
		},
		{
			Name:  schemas.EnvVarTargetDir,
			Value: TargetDir,
		},
		{
			Name:  schemas.EnvVarProfileName,
			Value: profileName,
		},
		{
			Name:  schemas.EnvVarPipelineID,
			Value: id.String(),
		},
		{
			Name:  schemas.EnvVarResultsDir,
			Value: ResultsDir,
		},
	}

	targetVolume := v1.Volume{
		Name: "target",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	resultsVolume := v1.Volume{
		Name: "results",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	secretVolume := v1.Volume{
		Name: secretVolumeName,
		VolumeSource: v1.VolumeSource{
			// we use a projected source over a secret source
			// to handle cases where the secret is not present or
			// a file already exists in the target directory
			Projected: &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{{
					Secret: &v1.SecretProjection{
						LocalObjectReference: v1.LocalObjectReference{
							Name: config.State.Secrets.SecretName,
						},
						Optional: ptr.Bool(true),
					},
				}},
			},
		},
	}

	volumes := []v1.Volume{
		targetVolume,
		secretVolume,
		resultsVolume,
	}

	targetVolumeMount := v1.VolumeMount{
		Name:      targetVolume.Name,
		MountPath: TargetDir,
	}

	resultsVolumeMount := v1.VolumeMount{
		Name:      resultsVolume.Name,
		MountPath: ResultsDir,
	}

	containerOpts := []runtime.ContainerOpt{
		runtime.ContainerEnvOpt(globalEnvVars...),
	}

	jobOpts := []runtime.JobOpt{
		runtime.JobOptWithVolumes(volumes),
	}

	var artifacts []string
	for _, artifact := range profile.Artifacts {
		if filepath.IsAbs(artifact) {
			artifacts = append(artifacts, filepath.Clean(artifact))
		} else {
			artifacts = append(artifacts, filepath.Join(ResultsDir, artifact))
		}
	}

	var us []uploaderInstance
	for _, uploaderDef := range profile.Uploaders {
		uplr, err := uploaderStorage.Get(ctx, uploaderDef.Name)
		if err != nil {
			return Pipeline{}, fmt.Errorf("error getting uploader %s: %w", uploaderDef.Name, err)
		}
		us = append(us, uploaderInstance{
			uplr, uploaderDef,
		})
	}

	var svcURL string
	if len(us) > 0 {
		uploaderJob := buildUploadJob(
			id,
			profileName,
			artifacts,
			us,
			append(
				containerOpts,
				runtime.ContainerVolumesOpt(resultsVolumeMount),
				runtime.ContainerWorkingDirOpt(ResultsDir),
			),
			jobOpts,
		)

		createdUploadJob, err := jobInterface.Create(ctx, &uploaderJob, metav1.CreateOptions{})
		if err != nil {
			return Pipeline{}, fmt.Errorf("error creating uploader job: %w", err)
		}

		l.Info("created uploader job", zap.String("job-name", createdUploadJob.Name))

		uploaderSvc := runtime.BuildServiceForJob(
			createdUploadJob,
			svcNamespace,
			extractorPort,
			// since the receiver is an init container the pod will not be "ready"
			// when the receiver is running. This means that (normally) the service
			// would not start routing traffic to the pod. We need to set this flag to
			// true to allow the service to route traffic to the pod even if it is not ready.
			runtime.ServiceOptPublishNotReadyAddresses(true),
		)

		_, err = serviceInterface.Create(ctx, &uploaderSvc, metav1.CreateOptions{})
		if err != nil {
			return Pipeline{}, fmt.Errorf("error creating uploader job: %w", err)
		}

		svcURL = fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			uploaderSvc.Name,
			uploaderSvc.Namespace,
			extractorPort,
		)
	} else {
		uploadStatus = schemas.RunStatusNotRan
	}

	scanJob := buildScanJob(
		id,
		profileName,
		target,
		dl,
		profile.Scanners,
		artifacts,
		append(containerOpts,
			runtime.ContainerVolumesOpt(targetVolumeMount, resultsVolumeMount),
			runtime.ContainerEnvOpt(v1.EnvVar{
				Name:  schemas.EnvVarUploaderHost,
				Value: svcURL,
			}),
		),
		jobOpts,
		len(us) == 0,
	)

	_, err := jobInterface.Create(ctx, &scanJob, metav1.CreateOptions{})
	if err != nil {
		return Pipeline{}, err
	}

	l.Info("created pipeline")

	return Pipeline{
		ID:           id,
		Profile:      profileName,
		Target:       target,
		ScanStatus:   scanStatus,
		UploadStatus: uploadStatus,
	}, nil
}

func buildScanJob(
	id uuid.UUID,
	profileName string,
	target schemas.Target,
	downloader resources.Downloader,
	scanners []schemas.Scanner,
	artifacts []string,
	containerOpts []runtime.ContainerOpt,
	jobOpts []runtime.JobOpt,
	ignoreUploaders bool,
) batchV1.Job {
	var extractorArgs runtime.ContainerOpt
	if ignoreUploaders {
		// in this case, no uploaders were defined in the profile, so no need to upload anything
		extractorArgs = runtime.ContainerArgsOpt(append([]string{"ignore", "--"}, artifacts...)...)
	} else {
		extractorArgs = runtime.ContainerArgsOpt(append([]string{"extract", "--"}, artifacts...)...)
	}

	initContainers := []v1.Container{
		runtime.CreateContainer(
			fmt.Sprintf("%s-downloader", target.Downloader),
			downloader.GetUserContainer(),
			secretVolumeName,
			append(containerOpts,
				runtime.ContainerWorkingDirOpt(TargetDir))...,
		),
		runtime.CreateContainer(
			"extract-artifacts",
			config.State.Extractor,
			secretVolumeName,
			append(
				containerOpts,
				extractorArgs,
				runtime.ContainerRestartPolicyOpt(v1.ContainerRestartPolicyAlways),
				runtime.ContainerWorkingDirOpt(ResultsDir),
			)...,
		),
	}

	var containers []v1.Container
	for i, scnr := range scanners {
		containers = append(containers,
			runtime.CreateContainer(
				fmt.Sprintf("scanner-%d", i),
				scnr,
				secretVolumeName,
				append(containerOpts,
					runtime.ContainerWorkingDirOpt(TargetDir))...,
			),
		)
	}

	if config.State.Runtime.ScannersServiceAccount != "" {
		jobOpts = append(
			jobOpts,
			runtime.JobOptWithServiceAccountName(config.State.Runtime.ScannersServiceAccount),
		)
	}

	return runtime.BuildJob(
		scanJobType,
		id,
		initContainers,
		containers,
		append(jobOpts,
			runtime.JobOptWithLabels(map[string]string{
				"pipeline-id":  id.String(),
				"profile-name": profileName,
			}),
			runtime.JobOptWithAnnotations(map[string]string{
				annotationPipelineID:       id.String(),
				annotationTargetDownloader: target.Downloader,
				annotationTargetIdentifier: target.Identifier,
				annotationTargetVersion:    target.Version,
				annotationProfileName:      profileName,
			}),
			runtime.JobOptWithTimeout(time.Minute*45),
		)...,
	)
}

func buildUploadJob(
	id uuid.UUID,
	profileName string,
	artifacts []string,
	uploaders []uploaderInstance,
	containerOpts []runtime.ContainerOpt,
	jobOpts []runtime.JobOpt,
) batchV1.Job {
	extractorPortStr := fmt.Sprintf("%d", extractorPort)

	uploadersInitContainers := []v1.Container{
		runtime.CreateContainer(
			"receive-artifacts",
			config.State.Extractor,
			secretVolumeName,
			append(
				containerOpts,
				runtime.ContainerEnvOpt(
					v1.EnvVar{Name: schemas.EnvVarExtractorPort, Value: extractorPortStr},
				),
				runtime.ContainerPortOpt(v1.ContainerPort{
					ContainerPort: extractorPort,
				}),
				runtime.ContainerArgsOpt(append([]string{"receive", "--"}, artifacts...)...),
			)...,
		),
	}

	var uploadContainers []v1.Container
	for i, u := range uploaders {
		uplrContainer := runtime.CreateContainer(
			fmt.Sprintf("uploader-%d", i),
			u.UserContainer,
			secretVolumeName,
			append(containerOpts,
				runtime.ContainerArgsOpt(append([]string{"--"}, artifacts...)...),
				runtime.ContainerEnvOpt(v1.EnvVar{Name: schemas.EnvVarUploaderName, Value: u.Name}),
				runtime.ContainerEnvOpt(
					runtime.EnvForParameters(
						u.UploaderRunRequest.Parameters,
						u.Uploader.Parameters,
					)...,
				),
			)...,
		)
		uploadContainers = append(uploadContainers, uplrContainer)
	}

	if config.State.Runtime.UploadersServiceAccount != "" {
		jobOpts = append(
			jobOpts,
			runtime.JobOptWithServiceAccountName(config.State.Runtime.UploadersServiceAccount),
		)
	}

	return runtime.BuildJob(
		uploadJobType,
		id,
		uploadersInitContainers,
		uploadContainers,
		append(jobOpts,
			runtime.JobOptWithLabels(map[string]string{
				"profile-name": profileName,
				"pipeline-id":  id.String(),
			}),
			runtime.JobOptWithAnnotations(map[string]string{
				annotationPipelineID:  id.String(),
				annotationProfileName: profileName,
			}),
			runtime.JobOptWithTimeout(time.Minute*45))...,
	)
}

type uploaderInstance struct {
	resources.Uploader
	schemas.UploaderRunRequest //nolint:govet
}
