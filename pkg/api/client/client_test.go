// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	standardClient := &http.Client{}
	tests := []struct {
		name           string
		baseURL        string
		c              *http.Client
		opts           []Opt
		validateClient func(*testing.T, *Client)
		wantErr        bool
	}{
		{
			name:    "valid client",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts:    nil,
			validateClient: func(t *testing.T, c *Client) {
				assert.Equal(t, standardClient, c.client.Client, "expected standard client")
				assert.Equal(
					t,
					"http://localhost:8080",
					c.client.serverHost,
					"expected base URL to match",
				)
			},
			wantErr: false,
		},
		{
			name:    "use default client",
			baseURL: "http://localhost:8080",
			c:       nil,
			opts:    nil,
			validateClient: func(t *testing.T, c *Client) {
				assert.Equal(t, http.DefaultClient, c.client.Client, "expceted default client")
			},
			wantErr: false,
		},
		{
			name:    "with context name option",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts: []Opt{
				WithContextName("test-context"),
			},
			validateClient: func(t *testing.T, c *Client) {
				assert.NotNil(t, c.client.Transport, "expected default client")
				if ctxTransport, ok := c.client.Transport.(*contextTransport); ok {
					assert.Equal(
						t,
						ctxTransport.ctx,
						"test-context",
						"expected context name to match",
					)
				}
			},
			wantErr: false,
		},
		{
			name:    "with token file option",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts: []Opt{
				TokenFileOpt("test-token-file", 10),
			},
			validateClient: func(t *testing.T, c *Client) {
				assert.NotNil(t, c.client.Transport, "expected transport to be set")
				if tokenTransport, ok := c.client.Transport.(*tokenFileTransport); ok {
					if tfRetriever, ok := tokenTransport.retriever.(tokenFileRetriever); ok {
						assert.Equal(
							t,
							"test-token-file",
							tfRetriever.file,
							"expected token file to match",
						)
						assert.Equal(
							t,
							time.Duration(10),
							tfRetriever.refresh,
							"expected token file to match",
						)
					} else {
						t.Fatal("expected transport retriever to be of type tokenFileRetriever")
					}
				} else {
					t.Fatal("expected transport to be of type tokenFileTransport")
				}
			},
		},
		{
			name:    "with static token option",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts: []Opt{
				StaticTokenOpt("test-token"),
			},
			validateClient: func(t *testing.T, c *Client) {
				assert.NotNil(t, c.client.Transport, "expected transport to be set")
				if tokenTransport, ok := c.client.Transport.(*tokenFileTransport); ok {
					if tfRetriever, ok := tokenTransport.retriever.(staticToken); ok {
						assert.Equal(
							t,
							"test-token",
							tfRetriever.token,
							"expected static token to match",
						)
					} else {
						t.Fatal("expected transport retriever to be of type staticToken")
					}
				} else {
					t.Fatal("expected transport to be of type tokenFileTransport")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.baseURL, tt.c, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("NewClient() got = %v, want non-nil", got)
			}

			if tt.validateClient != nil {
				tt.validateClient(t, got)
			}
		})
	}
}
