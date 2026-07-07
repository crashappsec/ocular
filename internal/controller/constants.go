// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	/* volumes */

	pipelineResultsVolumeName  = "ocular-pipeline-results"
	pipelineTargetVolumeName   = "ocular-pipeline-target"
	pipelineMetadataVolumeName = "ocular-pipeline-metadata"

	runtimeProcVolumeName   = "ocular-runtime-process"
	runtimeBinaryVolumeName = "ocular-runtime-binaries"

	searchTemplatesVolumeName = "ocular-search-templates"
	searchFIFOVolumeName      = "ocular-search-fifos"

	/* files */

	schedulerBinaryName = "scheduler"
	sidecarBinaryName   = "sidecar"

	pipelineTemplateName = "pipeline.template.json"
	pipelineFIFOName     = "pipelines"
	searchFIFOName       = "searches"

	/* directories */

	runtimeDirectory = "/var/run/ocular"
	processDirectory = runtimeDirectory + "/proc"
	fifoDirectory    = runtimeDirectory + "/fifo"
	binaryDirectory  = runtimeDirectory + "/bin"
	configDirectory  = "/etc/ocular"

	pipelineTargetDirectory   = "/mnt/target"
	pipelineResultsDirectory  = "/mnt/results"
	pipelineMetadataDirectory = "/mnt/metadata"

	/* path */

	pipelineTemplatePath = configDirectory + "/" + pipelineTemplateName
	sidecarBinaryPath    = binaryDirectory + "/" + sidecarBinaryName
	schedulerBinaryPath  = binaryDirectory + "/" + schedulerBinaryName

	pipelineFIFOPath = fifoDirectory + "/" + pipelineFIFOName
	searchFIFOPath   = fifoDirectory + "/" + searchFIFOName

	/* containers */

	uploadContainerPrefix   = "uploader-"
	scanContainerPrefix     = "scanner-"
	downloadContainerPrefix = "downloader-"

	crawlerContainerPrefix = "crawler-"

	sidecarInitContainerName   = "sidecar-init"
	schedulerInitContainerName = "scheduler-init"

	/* resource naming */

	pipelineResourcePrefix = "pipeline-"

	searchResourcePrefix = "search-"

	/* finalizers */

	// metricsFinalizer is a finalizer for
	// computing metrics on resources.
	metricsFinalizer = "ocular.crashoverride.run/metrics"
)

var (
	// sidecarInitResourceRequirements are the resource requirements
	// for the sidecar init container. This container is the first init
	// container of the pipeline pod, and just copies the binary to
	// a shared volume mount.
	sidecarInitResourceRequirements = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			// nolint:goconst
			"cpu": resource.MustParse("50m"),
			// nolint:goconst
			"memory": resource.MustParse("64Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("25m"),
			"memory": resource.MustParse("32Mi"),
		},
	}

	// schedulerInitResourceRequirements are the resource requirements
	// for the scheduler init container. This container is the first init
	// container of the search pod, and just copies the binary to
	// a shared volume mount.
	schedulerInitResourceRequirements = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			// nolint:goconst
			"cpu": resource.MustParse("50m"),
			// nolint:goconst
			"memory": resource.MustParse("64Mi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("25m"),
			"memory": resource.MustParse("32Mi"),
		},
	}

	// podStateChangedPredicate filters pod watch events to only
	// update when phase changed. Since Create/Delete are not
	// specified, they will be triggered for every create/delete
	podStateChangedPredicate = predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok1 := e.ObjectOld.(*corev1.Pod)
			newPod, ok2 := e.ObjectNew.(*corev1.Pod)
			if !ok1 || !ok2 {
				return true
			}

			return oldPod.Status.Phase != newPod.Status.Phase
			// we may need to check for when container status update
			// !equality.Semantic.DeepEqual(oldPod.Status.InitContainerStatuses, newPod.Status.InitContainerStatuses) ||
			// !equality.Semantic.DeepEqual(oldPod.Status.ContainerStatuses, newPod.Status.ContainerStatuses)
		},
	}
)
