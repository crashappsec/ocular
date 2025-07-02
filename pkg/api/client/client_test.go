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
)

func TestNewClient(t *testing.T) {
	standardClient := &http.Client{}
	tests := []struct {
		name           string
		baseURL        string
		c              *http.Client
		opts           []Opt
		validateClient func(*Client) error
		wantErr        bool
	}{
		{
			name:    "valid client",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts:    nil,
			validateClient: func(c *Client) error {
				if c.client.Client != standardClient {
					return fmt.Errorf("client is not given")
				}
				if c.client.serverHost != "http://localhost:8080" {
					return fmt.Errorf("baseURL is not set correctly")
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:    "use default client",
			baseURL: "http://localhost:8080",
			c:       nil,
			opts:    nil,
			validateClient: func(c *Client) error {
				if c.client.Client != http.DefaultClient {
					return fmt.Errorf("client is not default client")
				}
				return nil
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
			validateClient: func(c *Client) error {
				if c.client.Transport == nil {
					return fmt.Errorf("transport is nil")
				}
				if ctxTransport, ok := c.client.Transport.(*contextTransport); ok {
					if ctxTransport.ctx != "test-context" {
						return fmt.Errorf("context name is not set correctly")
					}
				} else {
					return fmt.Errorf("transport is not of type contextTransport")
				}
				return nil
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
			validateClient: func(c *Client) error {
				if c.client.Transport == nil {
					return fmt.Errorf("transport is nil")
				}
				if tokenTransport, ok := c.client.Transport.(*tokenFileTransport); ok {
					if tfRetriever, ok := tokenTransport.retriever.(tokenFileRetriever); ok {
						if tfRetriever.file != "test-token-file" {
							return fmt.Errorf("token file is not set correctly")
						}
						if tfRetriever.refresh != 10 {
							return fmt.Errorf("token refresh duration is not set correctly")
						}
					} else {
						return fmt.Errorf("retriever is not of type tokenFileRetriever")
					}
				} else {
					return fmt.Errorf("transport is not of type tokenFileTransport")
				}
				return nil
			},
		},
		{
			name:    "with static token option",
			baseURL: "http://localhost:8080",
			c:       standardClient,
			opts: []Opt{
				StaticTokenOpt("test-token"),
			},
			validateClient: func(c *Client) error {
				if c.client.Transport == nil {
					return fmt.Errorf("transport is nil")
				}
				if tokenTransport, ok := c.client.Transport.(*tokenFileTransport); ok {
					if tfRetriever, ok := tokenTransport.retriever.(staticToken); ok {
						if tfRetriever.token != "test-token" {
							return fmt.Errorf("static token is not set correctly")
						}
					} else {
						return fmt.Errorf("retriever is not of type staticToken")
					}
				} else {
					return fmt.Errorf("transport is not of type tokenFileTransport")
				}
				return nil
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
				if err := tt.validateClient(got); err != nil {
					t.Errorf("NewClient() validation error = %v", err)
				}
			}
		})
	}
}
