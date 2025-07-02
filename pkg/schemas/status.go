// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

import (
	"encoding/json"

	"github.com/swaggest/jsonschema-go"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

// RunStatus represents the status of a job run.
type RunStatus uint8

const (
	// RunStatusNotRan is used when the run has not been executed yet.
	RunStatusNotRan RunStatus = iota
	// RunStatusPending is used when the run is pending.
	RunStatusPending
	// RunStatusRunning is used when the run is running.
	RunStatusRunning
	// RunStatusSuccess is used when the run is successful.
	RunStatusSuccess
	// RunStatusFailure is used when the run has failed.
	RunStatusFailure
	// RunStatusCancelled is used when the run has been cancelled.
	RunStatusCancelled
	// RunStatusError is used when the run has encountered an error.
	// NOTE: this is different from failure, as it indicates that the run
	// was not able to complete due to an error, rather than a failure during the
	// execution of the run.
	RunStatusError
	// RunStatusUnknown is used when the status of the run is not known.
	RunStatusUnknown
)

const (
	// RunStatusPendingString is the string representation of RunStatusPending.
	RunStatusPendingString = "Pending"
	// RunStatusRunningString is the string representation of RunStatusRunning.
	RunStatusRunningString = "Running"
	// RunStatusSuccessString is the string representation of RunStatusSuccess.
	RunStatusSuccessString = "Success"
	// RunStatusFailureString is the string representation of RunStatusFailure.
	RunStatusFailureString = "Failure"
	// RunStatusCancelledString is the string representation of RunStatusCancelled.
	RunStatusCancelledString = "Cancelled"
	// RunStatusErrorString is the string representation of RunStatusError.
	RunStatusErrorString = "Error"
	// RunStatusNotRanString is the string representation of RunStatusNotRan.
	RunStatusNotRanString = "NotRan"
	// RunStatusUnknownString is the string representation of RunStatusUnknown.
	RunStatusUnknownString = "Unknown"
)

// RunStatusString returns a string representation of the run status.
func (r RunStatus) String() string {
	switch r {
	case RunStatusPending:
		return RunStatusPendingString
	case RunStatusRunning:
		return RunStatusRunningString
	case RunStatusSuccess:
		return RunStatusSuccessString
	case RunStatusFailure:
		return RunStatusFailureString
	case RunStatusCancelled:
		return RunStatusCancelledString
	case RunStatusError:
		return RunStatusErrorString
	case RunStatusNotRan:
		return RunStatusNotRanString
	case RunStatusUnknown:
		fallthrough
	default:
		return RunStatusUnknownString
	}
}

func (r RunStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + r.String() + `"`), nil
}

func (r *RunStatus) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	switch status {
	case RunStatusPendingString:
		*r = RunStatusPending
	case RunStatusRunningString:
		*r = RunStatusRunning
	case RunStatusSuccessString:
		*r = RunStatusSuccess
	case RunStatusFailureString:
		*r = RunStatusFailure
	case RunStatusCancelledString:
		*r = RunStatusCancelled
	case RunStatusErrorString:
		*r = RunStatusError
	case RunStatusNotRanString:
		*r = RunStatusNotRan
	case RunStatusUnknownString:
		fallthrough
	default:
		*r = RunStatusUnknown
	}

	return nil
}

func (r RunStatus) PrepareJSONSchema(schema *jsonschema.Schema) error {
	ty := &jsonschema.Type{}
	schema.Type = ty.WithSimpleTypes(jsonschema.String)

	// Add enum values for the RunStatus.
	schema.Enum = []interface{}{
		RunStatusPendingString,
		RunStatusRunningString,
		RunStatusSuccessString,
		RunStatusFailureString,
		RunStatusCancelledString,
		RunStatusErrorString,
		RunStatusUnknownString,
		RunStatusNotRanString,
	}

	schema.Description = ptr.To("The status of the run.")

	return nil
}

func (r RunStatus) MarshalYAML() (interface{}, error) {
	return r.String(), nil
}

func (r *RunStatus) UnmarshalYAML(value *yaml.Node) error {
	var status string
	if err := value.Decode(&status); err != nil {
		return err
	}

	switch status {
	case RunStatusPendingString:
		*r = RunStatusPending
	case RunStatusRunningString:
		*r = RunStatusRunning
	case RunStatusSuccessString:
		*r = RunStatusSuccess
	case RunStatusFailureString:
		*r = RunStatusFailure
	case RunStatusCancelledString:
		*r = RunStatusCancelled
	case RunStatusErrorString:
		*r = RunStatusError
	case RunStatusNotRanString:
		*r = RunStatusNotRan
	default:
		*r = RunStatusUnknown
	}

	return nil
}
