// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package client is a package that provides a client for making HTTP requests
// to the Ocular API. It handles authentication and token management.
// See the default crawler command for examples on how to use it.
package client

import (
	"net/http"
	"time"
)

// Client is a struct that represents a client for making HTTP requests
// to the Ocular API.
type Client struct {
	client *httpClient
}

// NewClient creates a new client for making HTTP requests to the Ocular API.
// baseURL is the base URL of the API server. c is an optional http.Client.
// Additional options are specified as a list of [Opt]
func NewClient(baseURL string, c *http.Client, opts ...Opt) (*Client, error) {
	client := &Client{
		client: newHTTPClient(c, baseURL),
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Opt is a function that configures A Client.
type Opt func(*Client) error

// WithContextName sets the context name for the client in requests.
var WithContextName = func(ctx string) Opt {
	return func(c *Client) error {
		c.client.Transport = &contextTransport{
			base: c.client.Transport,
			ctx:  ctx,
		}
		return nil
	}
}

// TokenFileOpt sets the token file for the client. The token is retrieved from the contents
// of the file, located at tokenFile. The token file is
// refreshed every refreshDuration.
var TokenFileOpt = func(tokenFile string, refreshDuration time.Duration) Opt {
	return func(c *Client) error {
		c.client.Transport = &tokenFileTransport{
			base: c.client.Transport,
			retriever: tokenFileRetriever{
				file:    tokenFile,
				refresh: refreshDuration,
			},
		}
		return nil
	}
}

// StaticTokenOpt sets a static token for the client. The token is used for
// authentication in requests. The token is not refreshed.
var StaticTokenOpt = func(token string) Opt {
	return func(c *Client) error {
		c.client.Transport = &tokenFileTransport{
			base: c.client.Transport,
			retriever: staticToken{
				token: token,
			},
		}
		return nil
	}
}
