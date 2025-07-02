// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utilities

import "testing"

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]string
		override map[string]string
		want     map[string]string
	}{
		{
			name:     "empty maps",
			base:     map[string]string{},
			override: map[string]string{},
			want:     map[string]string{},
		},
		{
			name:     "base empty",
			base:     map[string]string{},
			override: map[string]string{"key1": "value1"},
			want:     map[string]string{"key1": "value1"},
		},
		{
			name:     "override empty",
			base:     map[string]string{"key1": "value1"},
			override: map[string]string{},
			want:     map[string]string{"key1": "value1"},
		},
		{
			name:     "non-empty maps",
			base:     map[string]string{"key1": "value1", "key2": "value2"},
			override: map[string]string{"key2": "newValue2", "key3": "value3"},
			want:     map[string]string{"key1": "value1", "key2": "newValue2", "key3": "value3"},
		},
		{
			name:     "overriding existing keys",
			base:     map[string]string{"key1": "value1", "key2": "value2"},
			override: map[string]string{"key1": "newValue1", "key3": "value3"},
			want:     map[string]string{"key1": "newValue1", "key2": "value2", "key3": "value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeMaps(tt.base, tt.override); !equalMaps(t, got, tt.want) {
				t.Errorf("MergeMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalMaps(t *testing.T, a, b map[string]string) bool {
	t.Helper()
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if bVal, ok := b[k]; !ok || v != bVal {
			return false
		}
	}
	return true
}
