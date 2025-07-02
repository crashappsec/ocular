// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"context"
	"net/http"

	"github.com/crashappsec/ocular/pkg/schemas"
)

// CreatePipeline creates a new pipeline in the Ocular server.
// It will return a [responses.Pipeline] object on success or an error if the request fails.
func (c *Client) CreatePipeline(
	ctx context.Context,
	profile string,
	target schemas.Target,
) (schemas.Pipeline, error) {
	payload := schemas.PipelineRequest{
		ProfileName: profile,
		Target:      target,
	}

	pipeline, err := do[schemas.Pipeline](
		ctx,
		c.client,
		http.MethodPost,
		"/api/v1/pipelines",
		payload,
	)
	if err != nil {
		return schemas.Pipeline{}, err
	}

	return pipeline, nil
}
