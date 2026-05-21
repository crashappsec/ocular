// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

const (
	/* Volumes */

	pipelineResultsVolumeName  = "results"
	pipelineTargetVolumeName   = "target"
	pipelineMetadataVolumeName = "metadata"

	/* Ports */

	// extractorPort is the port that
	// that the ocular sidecar will run
	// listen for extracted artifacts
	extractorPort = 2121

	/* Naming */

	// scanSuffix is the suffix for
	// all scan resources created by a pipeline.
	// i.e. scan pod
	scanSuffix = "-scan"
	// uploadSuffix is the suffix for all
	// upload resources created by a pipeline
	// i.e. pod and service
	uploadSuffix = "-upload"

	// searchResourceSuffix is the suffix for
	// all search resources
	// i.e. search pod
	searchSuffix = "-search"

	/* Finalizers */

	// metricsFinalizer is a finalizer for
	// computing metrics on resources.
	metricsFinalizer = "ocular.crashoverride.run/metrics"
)
