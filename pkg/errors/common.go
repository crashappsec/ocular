// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package errors provides a way to create and handle errors with types and messages.
// Errors from this package are used to set the status code of API responses based on
// the [Type] of the [Error].
package errors

import (
	"errors"
	"fmt"
)

// Type represents the type of error.
// It should represent the status code that should be returned
// in the API response.
type Type = uint8

const (
	// TypeUnknown is used when the error type is not known.
	TypeUnknown Type = iota
	// TypeBadRequest is used when the request is invalid.
	TypeBadRequest
	// TypeNotFound is used when the requested resource is not found.
	TypeNotFound
	// TypeUnauthorized is used when the request is not authorized.
	TypeUnauthorized
	// TypeForbidden is used when the request is forbidden.
	TypeForbidden
)

// Error represents an error with a type and a message.
// It wraps the original error if one is provided.
type Error struct {
	Wrapped error
	Type    Type
	Message string
}

// New creates a new error with the given type, wrapped error, message and arguments.
// The message is formatted using [fmt.Sprintf] with the provided arguments.
func New(ty Type, wrapped error, msg string, args ...any) *Error {
	return &Error{
		Wrapped: wrapped,
		Type:    ty,
		Message: fmt.Sprintf(msg, args...),
	}
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return e.Wrapped.Error()
	}
	return e.Message
}

func (e *Error) Is(target error) bool {
	return errors.Is(e.Wrapped, target)
}

func (e *Error) Unwrap() error {
	return e.Wrapped
}
