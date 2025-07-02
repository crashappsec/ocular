// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package storage

import (
	"context"
	"encoding"
	"fmt"
	"strings"

	"github.com/crashappsec/ocular/pkg/cluster"
)

// dummyObject is an "example" implementation of the Object interface.
// It is mainly used for compile time checks to ensure that the Object interface is
// implemented correctly, and for testing purposes.
type dummyObject struct {
	value string
}

var (
	_ encoding.TextMarshaler   = dummyObject{}
	_ encoding.TextUnmarshaler = &dummyObject{}
)

func (e dummyObject) MarshalText() ([]byte, error) {
	return []byte("!!" + e.value), nil
}

func (e *dummyObject) UnmarshalText(data []byte) error {
	if !strings.HasPrefix(string(data), "!!") {
		return fmt.Errorf("invalid data: %s", string(data))
	}
	e.value = strings.TrimPrefix(string(data), "!!")
	return nil
}

func (e dummyObject) Validate(_ context.Context, _ cluster.Context) error {
	return nil
}
