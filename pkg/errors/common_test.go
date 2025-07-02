// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestError_Is(t *testing.T) {
	type testCase struct {
		name   string
		err    *Error
		target error
		want   bool
	}

	exampleError := fmt.Errorf("example error")

	tests := []testCase{
		{
			name:   "Is wrapped error",
			err:    New(TypeBadRequest, exampleError, "bad request"),
			target: exampleError,
			want:   true,
		},
		{
			name:   "Is not wrapped error",
			err:    New(TypeBadRequest, exampleError, "bad request"),
			target: fmt.Errorf("another error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Is(tt.target); got != tt.want {
				t.Errorf("Error.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_As(t *testing.T) {
	type testCase struct {
		name  string
		err   error
		valid bool
	}

	tests := []testCase{
		{
			name:  "As error",
			err:   New(TypeBadRequest, fmt.Errorf("some error"), "bad request"),
			valid: true,
		},
		{
			name:  "As not error",
			err:   fmt.Errorf("some error"),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e *Error
			if valid := errors.As(tt.err, &e); valid != tt.valid {
				t.Errorf("Error.As() = %v, want %v", valid, tt.valid)
			}
		})
	}
}
