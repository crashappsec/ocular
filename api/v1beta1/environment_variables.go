// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

// This file contains constants for environment variable names used in the Ocular system.
// These environment variables are set for containers running within Ocular pipelines or searches.
// They provide necessary configuration details such as target information, profile names, and directory paths.

type EnvironmentVariableName = string

const (
	/* Common environment variables */

	// EnvVarNamespaceName is the environment variable name for the name of the namespace
	// the pipeline or search is running in.
	EnvVarNamespaceName EnvironmentVariableName = "OCULAR_NAMESPACE_NAME"

	/* Pipeline and Profile related environment variables */

	// EnvVarTargetIdentifier is the environment variable name for the target identifier.
	// It specifies the unique identifier of the target being analyzed.
	EnvVarTargetIdentifier EnvironmentVariableName = "OCULAR_TARGET_IDENTIFIER"
	// EnvVarTargetVersion is the environment variable name for the target version.
	// It specifies the version of the target being analyzed.
	// Will be empty if not provided.
	EnvVarTargetVersion EnvironmentVariableName = "OCULAR_TARGET_VERSION"
	// EnvVarDownloaderName is the environment variable name for the downloader name.
	// It specifies the name of the [Downloader] resource used in the pipeline to fetch the target.
	EnvVarDownloaderName EnvironmentVariableName = "OCULAR_DOWNLOADER_NAME"
	// EnvVarProfileName is the environment variable name for the profile name.
	// It specifies the name of the [Profile] resource used in the pipeline to define extraction and analysis settings.
	EnvVarProfileName EnvironmentVariableName = "OCULAR_PROFILE_NAME"
	// EnvVarPipelineName is the environment variable name for the pipeline name.
	// It specifies the name of the [Pipeline] resource orchestrating the analysis process.
	EnvVarPipelineName EnvironmentVariableName = "OCULAR_PIPELINE_NAME"
	// EnvVarTargetDir is the environment variable name for the target directory.
	// It specifies the directory path where the target is downloaded and extracted within the container.
	// This variable is only set for [ProfileSpec.Containers] and not for [Uploader] containers.
	EnvVarTargetDir EnvironmentVariableName = "OCULAR_TARGET_DIR"
	// EnvVarResultsDir is the environment variable name for the results directory.
	// It specifies the directory path where analysis results should be stored within the container.
	// This variable is set for both [ProfileSpec.Containers] and [Uploader] containers.
	EnvVarResultsDir EnvironmentVariableName = "OCULAR_RESULTS_DIR"
	// EnvVarMetadataDir is the environment variable name for the metadata directory.
	// It specifies the directory path where target metadata files are stored within the container.
	// This variable is only set for [ProfileSpec.Containers] and not for [Uploader] containers.
	EnvVarMetadataDir EnvironmentVariableName = "OCULAR_METADATA_DIR"
	// EnvVarUploaderName is the environment variable name for the uploader name.
	// It specifies the name of the [Uploader] resource used in the pipeline to upload analysis results.
	EnvVarUploaderName EnvironmentVariableName = "OCULAR_UPLOADER_NAME"

	/* Search related environment variables */

	// EnvVarSearchName is the environment variable name for the search name.
	EnvVarSearchName EnvironmentVariableName = "OCULAR_SEARCH_NAME"

	// EnvVarCrawlerName is the environment variable name for the crawler name.
	EnvVarCrawlerName EnvironmentVariableName = "OCULAR_CRAWLER_NAME"

	// EnvVarPipelineFIFO is the environment variable that contains the path
	// to a named pipe (or FIFO) that will read JSON targets and automatically start pipelines
	// with the spec from the pipeline template in the search spec.
	EnvVarPipelineFIFO EnvironmentVariableName = "OCULAR_PIPELINE_FIFO"

	// EnvVarSearchFIFO is the environment variable that contains the path
	// to a named pipe (or FIFO) that will read JSON crawler references and automatically start
	// searches with the same scheduler configuration (pipeline template and interval)
	EnvVarSearchFIFO EnvironmentVariableName = "OCULAR_SEARCH_FIFO"

	// EnvVarPipelineTemplatePath is the name of the environment variable that contains
	// the path to the JSON data of the pipeline template to use when creating pipelines
	// from a search
	EnvVarPipelineTemplatePath EnvironmentVariableName = "OCULAR_PIPELINE_TEMPLATE"

	// EnvVarPipelineSchedulerIntervalSeconds is how long the scheduler should sleep between
	// creating pipelines
	EnvVarPipelineSchedulerIntervalSeconds EnvironmentVariableName = "OCULAR_PIPELINE_SCHEDULER_INTERVAL_SEC"

	// internal environment variables  //

	// EnvVarExtractorPort is the environment variable name for the extractor port.
	EnvVarSidecarExtractorPort EnvironmentVariableName = "OCULAR_SIDECAR_EXTRACTOR_PORT"

	// EnvVarExtractorHost is the environment variable name for the extractor host.
	EnvVarSidecarExtractorHost EnvironmentVariableName = "OCULAR_SIDECAR_EXTRACTOR_HOST"

	// EnvVarSidecarSchedulerCompletePath is the environment variable name for the path to
	// the file that is created when the crawler has completed
	EnvVarSidecarSchedulerCompletePath EnvironmentVariableName = "OCULAR_SIDECAR_SCHEDULER_COMPLETE_PATH"
)
