// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

type ResourceName = string

// Profile represents a series of scanners to run over a static asset
// and where to upload the results.
type Profile struct {
	// Scanners is a list of [Scanner] that will all be run
	// in parallel, with their current working directory set to
	// the directory where the target has been downloaded to.
	Scanners []Scanner `json:"scanners"  yaml:"scanners"  description:"A list of scanners that will be run over the target."`
	// Artifacts is a list of paths to the artifacts that will be produced
	// by the scanners. These paths are relative to the results directory
	Artifacts []string `json:"artifacts" yaml:"artifacts" description:"A list of paths to the artifacts that will be produced by the scanners. These paths are relative to the results directory."`
	// Uploaders is a list of [UploaderRunRequest] that will be used to upload
	// the results of the scanners. An uploader will be passed each of the artifacts
	// as command line arguments, prefixed by the argument '--' . Each [UploaderRunRequest] must specify the
	// name of the uploader and any parameters that are required.
	Uploaders []UploaderRunRequest `json:"uploaders" yaml:"uploaders" description:"A list of uploaders that will be used to upload the results of the scanners. An uploader will be passed each of the artifacts as command line arguments, prefixed by the argument '--'. Each UploaderRunRequest must specify the name of the uploader and any parameters that are required."`
}

// UploaderRunRequest represents an uploader that will be used to upload
type UploaderRunRequest struct {
	Name       string            `json:"name"                 yaml:"name"`
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// Scanner represents a scanner that will be run
// over the target. The scanner will be run in a container
// with the current working directory set to the directory
// where the target has been downloaded to.
type Scanner = UserContainer

// Downloader represents the init container that
// will download the static asset.
type Downloader = UserContainer

type Uploader = UserContainerWithParameters

// Crawler represents a crawler container configuration.
// It will serve as the base container configuration for crawler containers
// that are executed during a search. For more information on the configuration
// of the crawler container, see the [schemas.UserContainerWithParameters] type.
type Crawler = UserContainerWithParameters

// Secret is a type that represents a secret value.
// The byte slices contains the raw text of the secret.
type Secret []byte

func (s Secret) String() string {
	if s == nil {
		return "null"
	}
	return string(s)
}
