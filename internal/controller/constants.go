// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

const (
	/* Volumes */

	PipelineResultsVolumeName = "results"
	PipelineResultsDirectory  = "/mnt/results"

	PipelineTargetVolumeName = "target"
	PipelineTargetDirectory  = "/mnt/target"

	/* Ports */

	ExtractorPort = 2121

	/* Naming */

	scanPodSuffix        = "-scan"
	uploadPodSuffix      = "-upload"
	uploadServiceSuffix  = "-upload-svc"
	searchResourceSuffix = "-search"

	/* Labels */

	PipelineLabelKey   = "ocular.crashoverride.run/pipeline"
	SearchLabelKey     = "ocular.crashoverride.run/search"
	TypeLabelKey       = "ocular.crashoverride.run/type"
	ProfileLabelKey    = "ocular.crashoverride.run/profile"
	DownloaderLabelKey = "ocular.crashoverride.run/downloader"
	CrawlerLabelKey    = "ocular.crashoverride.run/crawler"

	PodTypeScan           = "scan"
	PodTypeUpload         = "upload"
	PodTypeSearch         = "search"
	ServiceTypeUpload     = "upload"
	ServiceTypeSearch     = "search"
	RoleBindingTypeSearch = "search"
)
