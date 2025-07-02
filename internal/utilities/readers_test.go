// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utilities

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/crashappsec/ocular/internal/unittest"
)

type errorReaderClose struct {
	err error
}

func (e errorReaderClose) Close() error {
	return e.err
}

func TestCloseAndLog(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "no error",
			err:  nil,
		},
		{
			name: "error",
			err:  fmt.Errorf("close error"),
		},
	}

	logs := unittest.CaptureLogs(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := errorReaderClose{err: tt.err}
			CloseAndLog(e)
			logBytes, err := io.ReadAll(logs)
			if err != nil {
				t.Fatalf("failed to read logs: %v", err)
			}
			if tt.err != nil {
				if !strings.Contains(string(logBytes), tt.err.Error()) {
					t.Errorf("expected error to be logged, got: %s", string(logBytes))
				}
			}
		})
	}
}

type errorReaderCloseWithType[T any] struct {
	err error
	val T
}

func (e errorReaderCloseWithType[T]) Close() (T, error) {
	return e.val, e.err
}

func TestCloseIgnoreAndLog(t *testing.T) {
	type test[T any] struct {
		name string
		err  error
		val  T
	}

	tests := []test[any]{
		{
			name: "no error",
			err:  nil,
			val:  "test",
		},
		{
			name: "error",
			err:  fmt.Errorf("close error"),
			val: map[string]interface{}{
				"key":  "value",
				"bool": true,
				"int":  42,
			},
		},
	}

	logs := unittest.CaptureLogs(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := errorReaderCloseWithType[any]{err: tt.err, val: tt.val}
			CloseIgnoreAndLog(e)
			logBytes, err := io.ReadAll(logs)
			if err != nil {
				t.Fatalf("failed to read logs: %v", err)
			}
			if tt.err != nil {
				if !strings.Contains(string(logBytes), tt.err.Error()) {
					t.Errorf("expected error to be logged, got: %s", string(logBytes))
				}
			}
		})
	}
}
