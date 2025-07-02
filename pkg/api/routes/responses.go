// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package routes

import (
	"errors"
	"net/http"

	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func WriteErr(c *gin.Context, err error) {
	zap.L().Debug("writing error response", zap.String("response_err", err.Error()))

	var target *errs.Error
	if errors.As(err, &target) {
		switch target.Type {
		case errs.TypeNotFound:
			WriteErrorResponse(c, http.StatusNotFound, schemas.ErrResourceNotFound, target.Message)
		case errs.TypeForbidden:
			WriteErrorResponse(c, http.StatusForbidden, schemas.ErrUnknown, target.Message)
		case errs.TypeBadRequest:
			WriteErrorResponse(c, http.StatusBadRequest, schemas.ErrInvalidPayload, target.Message)
		case errs.TypeUnauthorized:
			WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, target.Message)
		case errs.TypeUnknown:
			fallthrough
		default:
			WriteErrorResponse(
				c,
				http.StatusInternalServerError,
				schemas.ErrUnknown,
				target.Message,
			)
		}
		return
	}
	WriteErrorResponse(c, http.StatusInternalServerError, schemas.ErrUnknown, nil)
}

func WriteSuccessResponse(c *gin.Context, response any) {
	c.Negotiate(http.StatusOK, gin.Negotiate{
		Offered: []string{gin.MIMEJSON, gin.MIMEYAML},
		Data:    schemas.APIResponse[any]{Success: true, Response: response},
	})
}

func WriteErrorResponse(c *gin.Context, code int, err schemas.ErrorMsg, response any) {
	c.Abort()
	c.Negotiate(code, gin.Negotiate{
		Offered: []string{gin.MIMEJSON, gin.MIMEYAML},
		Data:    schemas.APIResponse[any]{Success: false, Error: err, Response: response},
	})
}
