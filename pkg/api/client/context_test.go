// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/crashappsec/ocular/pkg/schemas"
)

type validateContextTransport struct {
	expected string
}

func (v *validateContextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get(schemas.ClusterContextHeader) != v.expected {
		return nil, fmt.Errorf(
			"expected context header %s, got %s",
			v.expected,
			req.Header.Get(schemas.ClusterContextHeader),
		)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     req.Header,
	}, nil
}

func TestContextTransport_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		base       http.RoundTripper
		ctx        string
		wantHeader string
	}{
		{
			name:       "with base transport",
			base:       http.DefaultTransport,
			ctx:        "test-context-1",
			wantHeader: "test-context-1",
		},
		{
			name:       "without base transport",
			base:       nil,
			ctx:        "test-context-1",
			wantHeader: "test-context-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &contextTransport{
				base: &validateContextTransport{
					expected: tt.wantHeader,
				},
				ctx: tt.ctx,
			}
			req, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			_, err = c.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip() error = %v", err)
			}
		})
	}
}
