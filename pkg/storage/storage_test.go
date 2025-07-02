// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package storage

import (
	"strings"
	"testing"
)

func TestMarshalObject(t *testing.T) {
	tests := []struct {
		name string
		obj  any
		want []byte
	}{
		{
			name: "empty object",
			obj:  dummyObject{value: "testing"},
			want: []byte("!!testing"),
		},
		{
			name: "nil object",
			obj:  nil,
			want: nil,
		},
		{
			name: "string object",
			obj:  "test string",
			want: []byte("test string"),
		},
		{
			name: "int object",
			obj:  123,
			want: []byte("123"),
		},
		{
			name: "map object",
			obj:  map[string]string{"key": "value", "key2": "value2"},
			want: []byte("key: value\nkey2: value2\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := marshallObject(tt.obj)
			if err != nil {
				t.Errorf("marshallObject() error = %v", err)
				return
			}
			if strings.TrimSpace(string(got)) != strings.TrimSpace(string(tt.want)) {
				t.Errorf("marshallObject() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestUnmarshalObject(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		obj     any
		eqCheck func(result any) bool
		wantErr bool
	}{
		{
			name: "empty object",
			data: []byte("!!testing"),
			obj:  &dummyObject{value: "testing"},
			eqCheck: func(result any) bool {
				_, valid := result.(*dummyObject)
				return valid
			},
		},
		{
			name:    "failed empty object",
			data:    []byte("not empty object"),
			obj:     &dummyObject{},
			wantErr: true,
		},
		{
			name: "string object",
			data: []byte("test string"),
			obj:  new(string),
			eqCheck: func(result any) bool {
				return *result.(*string) == "test string"
			},
		},
		{
			name: "int object",
			data: []byte("123"),
			obj:  new(int),
			eqCheck: func(result any) bool {
				return *result.(*int) == 123
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unmarshallObject(tt.data, tt.obj)
			if !tt.wantErr && err != nil {
				t.Errorf("unmarshallObject() error = %v", err)
				return
			}

			if tt.eqCheck != nil && !tt.eqCheck(tt.obj) {
				t.Errorf("unmarshallObject() did not produce expected result")
			}
		})
	}
}
