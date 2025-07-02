// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/crashappsec/ocular/pkg/schemas"
)

type httpClient struct {
	*http.Client
	serverHost string
}

func newHTTPClient(baseClient *http.Client, host string) *httpClient {
	if baseClient == nil {
		baseClient = http.DefaultClient
	}

	return &httpClient{
		Client:     baseClient,
		serverHost: host,
	}
}

func do[Resp any](
	ctx context.Context,
	c *httpClient,
	method string,
	path string,
	payload any,
) (Resp, error) {
	requestURL := fmt.Sprintf(
		"%s/%s",
		strings.TrimSuffix(c.serverHost, "/"),
		strings.TrimPrefix(path, "/"),
	)
	var (
		resp schemas.APIResponse[Resp]
		body io.Reader
	)
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return resp.Response, fmt.Errorf("failed to marshal payload: %w", err)
		}

		body = bytes.NewReader(payloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return resp.Response, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ocular-client")
	respHTTP, err := c.Do(req)
	if err != nil {
		return resp.Response, fmt.Errorf("failed to do request: %w", err)
	}

	defer func() {
		_ = respHTTP.Body.Close()
	}()
	err = json.NewDecoder(respHTTP.Body).Decode(&resp)
	if err != nil {
		return resp.Response, fmt.Errorf("failed to decode response: %w", err)
	}

	if !resp.Success {
		return resp.Response, fmt.Errorf("error from server: %s", resp.Error)
	}

	return resp.Response, nil
}
