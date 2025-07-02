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

	"github.com/crashappsec/ocular/pkg/schemas"
)

type contextTransport struct {
	base http.RoundTripper
	ctx  string
}

func (c *contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(schemas.ClusterContextHeader, c.ctx)
	if c.base == nil {
		c.base = http.DefaultTransport
	}
	return c.base.RoundTrip(req)
}
