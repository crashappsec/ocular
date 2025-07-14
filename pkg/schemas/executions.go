// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

import (
	"github.com/google/uuid"
)

type ExecutionID = uuid.UUID

func ParseExecutionID(id string) (ExecutionID, error) {
	return uuid.Parse(id)
}

type Pipeline struct {
	// ID is the unique identifier for the pipeline execution.
	ID ExecutionID `json:"ID" yaml:"ID" profile:"id" description:"The unique identifier for the pipeline execution."`

	Profile string `json:"profile" yaml:"profile" description:"The profile to use for the pipeline execution."`

	// Target represents the target that the pipeline will run against.
	Target       Target    `json:"target"       yaml:"target"       description:"The target that the pipeline will run against."`
	ScanStatus   RunStatus `json:"scanStatus"   yaml:"scanStatus"   description:"The status of the pipeline execution."`
	UploadStatus RunStatus `json:"uploadStatus" yaml:"uploadStatus" description:"The status of the upload job execution."`
}

type ScheduledSearch struct {
	ID          ExecutionID       `yaml:"id,omitempty"       json:"id,omitempty"         description:"The unique identifier for the scheduled search."`
	Schedule    Schedule          `yaml:"schedule,omitempty" json:"schedule,omitempty"   description:"The cron schedule that the pipeline will run against." example:"0 0 * * *"`
	CrawlerName string            `yaml:"crawlerName"        json:"crawlerName"          description:"The name of the crawler to run."                       example:"example-crawler"`
	Parameters  map[string]string `yaml:"params,omitempty"   json:"parameters,omitempty" description:"The parameters to pass to the pipeline execution."`
}

type Schedule = string

type Search struct {
	CrawlerName string `json:"crawlerName"          yaml:"crawlerName"`
	// RunID is the ID of the run.
	ID ExecutionID `json:"runID"                yaml:"runID"`
	// Parameters is a map of parameter name to value.
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	// Status is the status of the run.
	Status RunStatus `json:"status"               yaml:"status"`
}
