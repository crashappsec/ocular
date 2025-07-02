// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

const (
	// EnvVarPrefix is the prefix used for environment variables
	EnvVarPrefix = "OCULAR_"
	// CustomEnvVarPrefix is the prefix used for environment variables
	// specified by the user that are already prefixed with EnvVarPrefix.
	CustomEnvVarPrefix = "CUSTOM_"

	// ParamEnvVarPrefix is the prefix used for environment variables
	// that contain parameters for uploader or crawler invocations.
	ParamEnvVarPrefix = EnvVarPrefix + "PARAM_"
)

/**********************************
 * Pipeline Environment Variables *
 **********************************/

const (
	EnvVarTargetDownloader = EnvVarPrefix + "TARGET_DOWNLOADER"
	EnvVarTargetIdentifier = EnvVarPrefix + "TARGET_IDENTIFIER"
	EnvVarTargetVersion    = EnvVarPrefix + "TARGET_VERSION"
	EnvVarTargetDir        = EnvVarPrefix + "TARGET_DIR"
	EnvVarResultsDir       = EnvVarPrefix + "RESULTS_DIR"
	EnvVarProfileName      = EnvVarPrefix + "PROFILE_NAME"
	EnvVarPipelineID       = EnvVarPrefix + "PIPELINE_ID"

	EnvVarUploaderHost  = EnvVarPrefix + "UPLOADER_HOST"
	EnvVarExtractorPort = EnvVarPrefix + "EXTRACTOR_PORT"
	EnvVarUploaderName  = EnvVarPrefix + "UPLOADER_NAME"
)

/**********************************
 * Search Environment Variables *
 **********************************/

const (
	EnvVarOcularTokenPath = EnvVarPrefix + "SERVICE_ACCOUNT_TOKEN_PATH" // #nosec G101

	EnvVarCrawlerName = EnvVarPrefix + "CRAWLER_NAME"
	EnvVarContextName = EnvVarPrefix + "CONTEXT_NAME"
	EnvVarAPIBaseURL  = EnvVarPrefix + "API_BASE_URL"
)
