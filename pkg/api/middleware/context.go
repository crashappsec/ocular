// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package middleware provides middleware for the Ocular API.
// It includes authentication, authorization, and other middleware
package middleware

import (
	"context"
	"net/http"

	"github.com/crashappsec/ocular/pkg/api/routes"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ContextAssigner is a gin middleware that assigns the cluster context to the request context.
// It checks the "X-ClusterContext-Name" header for the context name.
// If the header is not set, it will use the default context if enabled.
// The context assigned is the one that is used for authorization and
// for interacting with the cluster.
func ContextAssigner(manager cluster.ContextManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L()
		contextHeader := c.GetHeader(schemas.ClusterContextHeader)
		if contextHeader == "" {
			l.Debug("no X-ClusterContext-Name header, using default")
			defaultCtx, enabled := manager.DefaultContext()
			if enabled {
				l.Debug(
					"using default context",
					zap.String("defaultContext", defaultCtx.Name),
					zap.String("defaultContextNamespace", defaultCtx.Namespace),
				)
				SetClusterContext(c, defaultCtx)
				return
			}
			l.Debug("no X-ClusterContext-Name header, and no default")
			routes.WriteErrorResponse(
				c,
				http.StatusBadRequest,
				schemas.ErrDefaultContextNotEnabled,
				nil,
			)
		} else {
			l.Debug("using context from header", zap.String("contextHeader", contextHeader))
			ctx, exists := manager.GetContext(contextHeader)
			if exists {
				l.Debug("found context from header", zap.String("contextHeader", contextHeader))
				SetClusterContext(c, ctx)
			} else {
				l.Debug("given cluster context does not exist", zap.String("contextHeader", contextHeader))
				routes.WriteErrorResponse(c, http.StatusBadRequest, schemas.ErrDefaultContextNotEnabled, nil)
			}
		}
	}
}

const clusterContextKey = "clusterContext"

// GetClusterContext retrieves the cluster context from the request context.
func GetClusterContext(c context.Context) (cluster.Context, bool) {
	ctxVal := c.Value(clusterContextKey)

	if ctxVal == nil {
		return cluster.Context{}, false
	}

	ctx, valid := ctxVal.(cluster.Context)
	return ctx, valid
}

// SetClusterContext sets the cluster context in the request context.
func SetClusterContext(c *gin.Context, ctx cluster.Context) {
	c.Set("clusterContext", ctx)
}

// MustClusterContext retrieves the cluster context from the request context.
// It panics if the context is not found or is not of type cluster.Context.
func MustClusterContext(c *gin.Context) cluster.Context {
	ctxVal := c.Value("clusterContext")

	if ctxVal == nil {
		zap.L().Debug("context not found")
		routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
		panic("no cluster context found")
	}

	ctx, valid := ctxVal.(cluster.Context)
	if !valid {
		zap.L().Warn("context is not of type cluster.Context")
		routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
		panic("no cluster context found")
	}

	return ctx
}
