// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package unittest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

// AssertEqualContainers compares two [k8s.io/api/core/v1.Container] objects for equality.
// It marshals both containers to JSON and compares the resulting strings.
func AssertEqualContainers(t *testing.T, expected, actual v1.Container) {
	t.Helper()
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("failed to marshal actual container: %v", err)
	}

	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("failed to marshal expected container: %v", err)
	}

	assert.JSONEq(t, string(expectedJSON), string(actualJSON), "Containers are not equal")
}
