// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestStaticToken_GetToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "valid token",
			token: "test-token",
		},
		{
			name:  "empty token",
			token: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := staticToken{
				token: tt.token,
			}
			gotToken, _, err := s.GetToken()
			if err != nil {
				t.Errorf("GetToken() error = %v", err)
				return
			}
			if gotToken != tt.token {
				t.Errorf("GetToken() = %v, want %v", gotToken, tt.token)
			}
		})
	}
}

func TestTokenFileRetriever_GetToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-token-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	tmpDir = strings.TrimSuffix(tmpDir, "/")
	tests := []struct {
		name    string
		file    string
		refresh time.Duration
	}{
		{
			name:    "valid token file",
			file:    tmpDir + "/test-1.txt",
			refresh: 10 * time.Second,
		},
	}
	// Create a temporary file to test with
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tt.file, []byte("test-token"), 0o600); err != nil {
				t.Fatalf("failed to create test token file: %v", err)
			}
			defer func() {
				_ = os.Remove(tt.file)
			}()

			tr := tokenFileRetriever{
				file:    tt.file,
				refresh: tt.refresh,
			}
			gotToken, nextCheck, err := tr.GetToken()
			if err != nil {
				t.Errorf("GetToken() error = %v", err)
				return
			}
			if gotToken != "test-token" {
				t.Errorf("GetToken() = %v, want %v", gotToken, "test-token")
			}
			wait := time.Until(nextCheck)
			diff := tt.refresh - wait
			if diff < 0 || diff > time.Second {
				t.Errorf(
					"nextCheck did not match expected refresh duration within 1 second, got %v, want %v",
					wait,
					tt.refresh,
				)
			}
		})
	}
}
