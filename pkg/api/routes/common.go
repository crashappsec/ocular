// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package routes (and sub-packages) provides the API routes for the Ocular application.
package routes

import (
	"net/http"

	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MustParam retrieves a parameter from the URL path and checks if it is empty.
// If it is empty, it writes an error response and returns false.
func MustParam(c *gin.Context, key string) (string, bool) {
	name := c.Param(key)
	if name == "" {
		zap.L().Debug(c.FullPath() + " name not provided")
		WriteErrorResponse(
			c,
			http.StatusBadRequest,
			schemas.ErrInvalidParameter,
			key+" is required in path "+c.FullPath(),
		)
		return "", false
	}
	return name, true
}

// Health returns a handler function that responds with a success message.
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		WriteSuccessResponse(c, "ok")
	}
}

func ServeStatic(content []byte, contentType string) gin.HandlerFunc {
	if contentType == "" {
		contentType = "application/text"
	}
	return func(c *gin.Context) {
		c.Data(http.StatusOK, contentType, content)
	}
}

func Version(version, buildTime, commit string) gin.HandlerFunc {
	return func(c *gin.Context) {
		WriteSuccessResponse(c,
			schemas.APIVersionResponse{
				Version:   version,
				BuildTime: buildTime,
				Commit:    commit,
			})
	}
}
