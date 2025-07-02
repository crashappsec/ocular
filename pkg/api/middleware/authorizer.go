// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package middleware

import (
	"net/http"

	"github.com/crashappsec/ocular/pkg/api/routes"
	"github.com/crashappsec/ocular/pkg/identities"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthorizeIdentity is a middleware that checks if the user is authorized to access the resource.
// It will check if the identities.Identity from the request is authorized by calling each identity.Authorizer
// provided. If any of the authorizers return true, the user is authorized.
func AuthorizeIdentity(authorizers ...identities.Authorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterCtx, exists := GetClusterContext(c)
		if !exists {
			// no cluster found THIS SHOULD NOT OCCUR
			routes.WriteErrorResponse(
				c,
				http.StatusUnauthorized,
				schemas.ErrUnauthenticated,
				nil,
			)
			return
		}

		identity, exists := GetIdentity(c)
		if !exists {
			// no identity found
			routes.WriteErrorResponse(
				c,
				http.StatusUnauthorized,
				schemas.ErrUnauthenticated,
				nil,
			)
			return
		}
		l := zap.L().With(
			zap.Any("identity", identity),
			zap.Int("authorizers_amount", len(authorizers)),
		)

		for _, authorizer := range authorizers {
			authorized, err := authorizer(c, clusterCtx, identity)
			if err != nil {
				l.Error("error while attempting to authorize user", zap.Error(err))
				routes.WriteErrorResponse(
					c, http.StatusUnauthorized, schemas.ErrUnauthenticated,
					"unknown error while authorizing user",
				)
				return
			}
			if authorized {
				l.Debug("user authorized")
				return
			}
		}

		l.Debug("user is not authorized to access resource")
		routes.WriteErrorResponse(c, http.StatusForbidden, schemas.ErrUnauthorized, nil)
	}
}
