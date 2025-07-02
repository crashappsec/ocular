// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package schemas provides types that will be used in the API
// and marshalled or unmarshalled from user data
package schemas

// PipelineRequest represents a request to run a pipeline
type PipelineRequest struct {
	ProfileName string `json:"profileName" yaml:"profileName"`
	Target      Target `json:"target"      yaml:"target"`
}

// ScheduleRequest represents a request to schedule a search
type ScheduleRequest struct {
	CrawlerName string            `json:"crawlerName" yaml:"crawlerName"`
	Schedule    string            `json:"schedule"    yaml:"schedule"`
	Parameters  map[string]string `json:"parameters"  yaml:"parameters"`
}

// SearchRequest represents a request to run a search
type SearchRequest struct {
	CrawlerName string            `json:"crawlerName" yaml:"crawlerName"`
	Parameters  map[string]string `json:"parameters"  yaml:"parameters"`
}
