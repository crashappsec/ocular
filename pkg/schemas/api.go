// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package schemas

type ErrorMsg = string

const (
	/* Standard error messages */

	// ErrUnknown is a generic error message used when the error type is not known.
	ErrUnknown ErrorMsg = "unknown error"
	// ErrInvalidPayload is returned when the payload is invalid and cannot be parsed.
	ErrInvalidPayload    ErrorMsg = "invalid payload, unable to parse"
	ErrInvalidParameter  ErrorMsg = "invalid parameter, unable to parse"
	ErrInvalidIdentifier ErrorMsg = "invalid identifier, unable to parse"
	ErrResourceNotFound  ErrorMsg = "resource not found"

	/* System Configuration errors */

	// ErrDefaultContextNotEnabled is returned when no context is set and
	// the default context is not enabled in the system configuration.
	ErrDefaultContextNotEnabled ErrorMsg = "no context set and default context is not enabled"

	/* Authentication errors */

	// ErrInvalidAuthenticationHeader is returned when the authentication header is invalid.
	ErrInvalidAuthenticationHeader ErrorMsg = "invalid authentication header"
	// ErrInvalidTokenHeader is returned when the bearer token header is invalid.
	ErrInvalidTokenHeader ErrorMsg = "invalid bearer token"
	// ErrUnauthenticated is returned when the user is not authenticated.
	ErrUnauthenticated ErrorMsg = "unable to authenticate"
	// ErrUnauthorized is returned when the user is not authorized to access the resource.
	ErrUnauthorized ErrorMsg = "unauthorized to access resource"
)

const (
	// ClusterContextHeader is the header used to pass the cluster context name in requests.
	ClusterContextHeader = "X-ClusterContext-Name"
)

type APIResponse[T any] struct {
	Success  bool     `json:"success"            yaml:"success"`
	Error    ErrorMsg `json:"error,omitempty"    yaml:"error,omitempty"`
	Response T        `json:"response,omitempty" yaml:"response,omitempty"`
}

type APIVersionResponse struct {
	Version   string `json:"version"             yaml:"version"`
	BuildTime string `json:"buildTime,omitempty" yaml:"buildTime,omitempty"`
	Commit    string `json:"commit,omitempty"    yaml:"commit,omitempty"`
}
