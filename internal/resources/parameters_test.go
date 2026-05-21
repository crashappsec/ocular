// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"fmt"
	"testing"

	"github.com/crashappsec/ocular/api/v1beta1"
)

func TestParseParameters(t *testing.T) {
	tests := []struct {
		name     string
		defs     []v1beta1.ParameterDefinition
		settings []v1beta1.ParameterSetting
		parent   map[string]string
		expected map[string]string
	}{
		{
			name:     "empty",
			parent:   nil,
			expected: map[string]string{},
		},
		{
			name: "standard",
			defs: []v1beta1.ParameterDefinition{
				{
					Name: "REQUIRED",
				},
				{
					Name:    "NOT_REQUIRED",
					Default: new("not-set"),
				},
				{
					Name:    "DEFAULT",
					Default: new("default-value"),
				},
			},
			settings: []v1beta1.ParameterSetting{
				{
					Name:  "REQUIRED",
					Value: "required-set",
				},
				{
					Name:  "NOT_REQUIRED",
					Value: "set",
				},
				{
					Name:  "UNSPECIFIED",
					Value: "ignored",
				},
			},
			expected: map[string]string{
				"REQUIRED":     "required-set",
				"NOT_REQUIRED": "set",
				"DEFAULT":      "default-value",
			},
		},
		{
			name: "standard",
			defs: []v1beta1.ParameterDefinition{
				{
					Name:    "PARENT",
					Default: new("default-value"),
				},
			},
			settings: []v1beta1.ParameterSetting{
				{
					Name: "PARENT",
					ValueFrom: &v1beta1.ParameterSource{
						ParentParam: "PARENT",
					},
				},
			},
			parent: map[string]string{
				"PARENT": "override",
			},
			expected: map[string]string{
				"PARENT": "override",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseParameters(tt.defs, tt.settings, tt.parent)
			if err := equalMaps(t, got, tt.expected); err != nil {
				t.Errorf("ParseParameters() = %v, want %v: %s", got, tt.expected, err)
			}
		})
	}

}

func equalMaps(t *testing.T, a, b map[string]string) error {
	t.Helper()
	if len(a) != len(b) {
		return fmt.Errorf("lengths do not match got %d, expected %d", len(a), len(b))
	}

	for k, v := range a {
		bVal, ok := b[k]
		if !ok {
			return fmt.Errorf("got key %s (with value %s) but did not exist in expected", k, v)
		}
		if v != bVal {
			return fmt.Errorf("got key %s with value %s, but expected %s", k, v, bVal)
		}
	}
	return nil
}
