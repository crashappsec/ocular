// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1

import (
	"net/http"
	"strings"

	"github.com/crashappsec/ocular/pkg/api/middleware"
	"github.com/crashappsec/ocular/pkg/api/routes"
	"github.com/crashappsec/ocular/pkg/pipelines"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RunPipeline() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "RunPipeline"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		l.Debug("creating pipeline for context", zap.String("context", clusterCtx.Name))

		var req schemas.PipelineRequest
		if err := c.ShouldBind(&req); err != nil {
			routes.WriteErrorResponse(
				c,
				http.StatusBadRequest,
				schemas.ErrInvalidPayload,
				err.Error(),
			)
			return
		}

		dl, err := resources.NewDownloaderStorageBackend(clusterCtx).Get(
			c,
			req.Target.Downloader,
		)
		if err != nil {
			l.Warn("error getting downloader", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		profile, err := resources.NewProfileStorageBackend(clusterCtx).Get(c, req.ProfileName)
		if err != nil {
			l.Error(
				"unable to retrieve profile",
				zap.String("profile", req.ProfileName),
				zap.Error(err),
			)
			routes.WriteErr(c, err)
			return
		}

		uploaderStorage := resources.NewUploaderStorageBackend(clusterCtx)

		pipeline, err := pipelines.Run(
			c,
			clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace),
			clusterCtx.CS.CoreV1().Services(clusterCtx.Namespace),
			clusterCtx.Namespace,
			req.Target,
			dl,
			req.ProfileName,
			profile,
			uploaderStorage,
		)
		if err != nil {
			l.Error("error creating pipeline", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		routes.WriteSuccessResponse(c, pipeline)
	}
}

func DeletePipeline() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "DeletePipeline"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		idStr := strings.Trim(c.Param("id"), "/")

		id, err := schemas.ParseExecutionID(idStr)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}

		l.Debug(
			"deleting pipeline for context",
			zap.String("context", clusterCtx.Name),
			zap.String("id", id.String()),
		)

		err = pipelines.Stop(c, clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace), id)
		if err != nil {
			l.Error("error deleting pipeline", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, nil)
	}
}

func GetPipeline() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "GetPipeline"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		idStr := strings.Trim(c.Param("id"), "/")

		id, err := schemas.ParseExecutionID(idStr)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}

		l.Debug(
			"retrieving pipeline for context",
			zap.String("context", clusterCtx.Name),
			zap.String("id", id.String()),
		)

		pipeline, err := pipelines.Get(c, clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace), id)
		if err != nil {
			l.Error("error deleting pipeline", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		routes.WriteSuccessResponse(c, pipeline)
	}
}

func ListPipelines() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "ListPipelines"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		l.Debug("listing pipelines for context", zap.String("context", clusterCtx.Name))

		plines, err := pipelines.List(c, clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace))
		if err != nil {
			l.Error("error deleting pipeline", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		routes.WriteSuccessResponse(
			c,
			plines,
		)
	}
}
