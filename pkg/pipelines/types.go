// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package pipelines

import (
	"github.com/crashappsec/ocular/pkg/schemas"
)

const (
	TargetDir        = "/mnt/target"
	ResultsDir       = "/mnt/results"
	secretVolumeName = "pipeline-secrets"

	scanJobType   = "scan"
	uploadJobType = "upload"

	extractorPort = 2121
)

type Pipeline schemas.Pipeline

/* Annotation Labels */
const (
	annotationTargetDownloader = "crashoverride.run/target-downloader"
	annotationTargetIdentifier = "crashoverride.run/target-identifier"
	annotationTargetVersion    = "crashoverride.run/target-version"
	annotationProfileName      = "crashoverride.run/profile-name"
	annotationPipelineID       = "crashoverride.run/pipeline-id"
)
