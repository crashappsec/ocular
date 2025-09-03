// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1

// This file contains constants for environment variable names used in the Ocular system.
// These environment variables are set for containers running within Ocular pipelines or searches.
// They provide necessary configuration details such as target information, profile names, and directory paths.

type EnvironmentVariableName = string

const (
	// EnvVarOcularTargetIdentifier is the environment variable name for the target identifier.
	// It specifies the unique identifier of the target being analyzed.
	EnvVarOcularTargetIdentifier EnvironmentVariableName = "OCULAR_TARGET_IDENTIFIER"
	// EnvVarOcularTargetVersion is the environment variable name for the target version.
	// It specifies the version of the target being analyzed.
	// Will be empty if not provided.
	EnvVarOcularTargetVersion EnvironmentVariableName = "OCULAR_TARGET_VERSION"
	// EnvVarOcularDownloaderName is the environment variable name for the downloader name.
	// It specifies the name of the [Downloader] resource used in the pipeline to fetch the target.
	EnvVarOcularDownloaderName EnvironmentVariableName = "OCULAR_DOWNLOADER_NAME"
	// EnvVarOcularProfileName is the environment variable name for the profile name.
	// It specifies the name of the [Profile] resource used in the pipeline to define extraction and analysis settings.
	EnvVarOcularProfileName EnvironmentVariableName = "OCULAR_PROFILE_NAME"
	// EnvVarOcularPipelineName is the environment variable name for the pipeline name.
	// It specifies the name of the [Pipeline] resource orchestrating the analysis process.
	EnvVarOcularPipelineName EnvironmentVariableName = "OCULAR_PIPELINE_NAME"
	// EnvVarOcularTargetDir is the environment variable name for the target directory.
	// It specifies the directory path where the target is downloaded and extracted within the container.
	// This variable is only set for [ProfileSpec.Containers] and not for [Uploader] containers.
	EnvVarOcularTargetDir EnvironmentVariableName = "OCULAR_TARGET_DIR"
	// EnvVarOcularResultsDir is the environment variable name for the results directory.
	// It specifies the directory path where analysis results should be stored within the container.
	// This variable is set for both [ProfileSpec.Containers] and [Uploader] containers.
	EnvVarOcularResultsDir EnvironmentVariableName = "OCULAR_RESULTS_DIR"

	// internal environment variables  //

	// EnvVarExtractorPort is the environment variable name for the extractor port.
	EnvVarExtractorPort EnvironmentVariableName = "EXTRACTOR_PORT"

	// EnvVarExtractorHost is the environment variable name for the extractor host.
	EnvVarExtractorHost EnvironmentVariableName = "EXTRACTOR_HOST"
)
