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
	"strings"

	"github.com/crashappsec/ocular/pkg/api/routes"
	"github.com/crashappsec/ocular/pkg/identities"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

// BearerAuthenticator is a gin middleware that authenticates the user using a bearer token.
// It checks the "Authorization" header for a bearer token and verifies it.
// If the token is valid, it sets the "identity" key in the context with the user information.
// If the token is invalid, it returns a 401 Unauthorized response.
func BearerAuthenticator() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L()
		clusterCtx, exists := GetClusterContext(c)
		if !exists {
			// no cluster found THIS SHOULD NOT OCCUR
			l.Warn("could not find cluster context")
			return
		}

		_, exists = c.Get("identity")
		if exists {
			l.Debug("identity already set for request")
			return // return if already set
		}

		authHeader := c.GetHeader("Authorization")
		// header not set, skip since it may be user cert
		if authHeader == "" {
			l.Debug("identity already set for request")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			l.Debug("invalid authorization header")
			routes.WriteErrorResponse(c, 400, schemas.ErrInvalidAuthenticationHeader, nil)
			return
		}
		token := parts[1]

		userInfo, err := identities.AuthenticateToken(
			c,
			clusterCtx.CS.AuthenticationV1().TokenReviews(),
			token,
		)
		if err != nil {
			l.Debug("could not authenticate token", zap.Error(err))
			routes.WriteErrorResponse(
				c,
				http.StatusUnauthorized,
				schemas.ErrInvalidTokenHeader,
				nil,
			)
			return
		}

		c.Set("identity", userInfo)
	}
}

// CertificateAuthenticator is a gin middleware that authenticates the user using a client certificate.
// It checks the TLS connection for client certificates and verifies them.
func CertificateAuthenticator() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().Sugar()
		clusterCtx, exists := GetClusterContext(c)
		if !exists {
			// no cluster found THIS SHOULD NOT OCCUR
			l.Warn("could not find cluster context")
			return
		}

		_, exists = c.Get("identity")
		if exists {
			l.Debug("identity already set for request")
			return // return if already set
		}

		// ignore non TLS connection
		if c.Request.TLS == nil {
			l.Debug("no TLS certificate provided")
			return
		}

		peerCertificates := c.Request.TLS.PeerCertificates
		// no peer certificates provided
		if len(peerCertificates) == 0 {
			l.Debug("no peer certificate provided")
			return
		}

		var result *multierror.Error
		for _, cert := range peerCertificates {
			identity, err := identities.AuthenticateClient(c, clusterCtx, cert)
			if err == nil {
				c.Set("identity", identity)
				return
			}
			result = multierror.Append(result, err)
		}

		if err := result.ErrorOrNil(); err != nil {
			l.Debug("no peer certificates were able to be authenticated", zap.Error(err))
		}
	}
}

// GetIdentity retrieves the identity from the context.
func GetIdentity(c *gin.Context) (identities.Identity, bool) {
	identity, exists := c.Get("identity")
	if !exists {
		return identities.Identity{}, false
	}
	return identity.(identities.Identity), true
}
