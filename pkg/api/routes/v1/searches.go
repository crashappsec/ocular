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

	"github.com/crashappsec/ocular/pkg/api/middleware"
	"github.com/crashappsec/ocular/pkg/api/routes"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/searches"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RunSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "RunSearch"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}
		var searchRequest schemas.SearchRequest
		if err := c.ShouldBind(&searchRequest); err != nil {
			routes.WriteErrorResponse(
				c,
				http.StatusBadRequest,
				schemas.ErrInvalidPayload,
				"invalid request body, must be a JSON object with string keys and values",
			)
			return
		}

		l.Debug("search request", zap.Any("searchRequest", searchRequest))
		crawler, err := resources.NewCrawlerStorageBackend(clusterCtx).
			Get(c, searchRequest.CrawlerName)
		if err != nil {
			l.Error("error getting crawler", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		l.Debug("crawler found, validating parameters", zap.Any("crawler", crawler))
		if err = resources.ValidateParameters(searchRequest.Parameters, crawler.Parameters); err != nil {
			l.Error("error getting crawler", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		run, err := searches.Run(
			c,
			clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace),
			clusterCtx.Name,
			searchRequest.CrawlerName,
			crawler,
			searchRequest.Parameters,
		)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, run)
	}
}

func StopSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "SetCrawler"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		id, err := searches.ParseRunID(c.Param("id"))
		if err != nil {
			routes.WriteErr(c, err)
			return
		}

		err = searches.Stop(
			c,
			clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace),
			id,
		)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, nil)
	}
}

func ListSearches() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "ListSearches"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		runs, err := searches.List(
			c,
			clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace),
		)
		if err != nil {
			l.Error("error getting crawler", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, runs)
	}
}

func GetSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "GetSearch"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		id, err := searches.ParseRunID(c.Param("id"))
		if err != nil {
			routes.WriteErr(c, err)
			return
		}

		run, err := searches.Get(
			c,
			clusterCtx.CS.BatchV1().Jobs(clusterCtx.Namespace),
			id,
		)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, run)
	}
}

func ScheduleSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "ScheduleSearch"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		var req schemas.ScheduleRequest
		if err := c.ShouldBind(&req); err != nil {
			routes.WriteErrorResponse(
				c,
				http.StatusBadRequest,
				schemas.ErrInvalidPayload,
				"invalid request body for schedule request",
			)
			return
		}

		crawler, err := resources.NewCrawlerStorageBackend(clusterCtx).Get(c, req.CrawlerName)
		if err != nil {
			l.Error("error getting crawler", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		l.Debug(
			"setting schedule for crawler",
			zap.String("context", clusterCtx.Name),
			zap.String("crawler_name", req.CrawlerName),
		)

		if err = resources.ValidateParameters(req.Parameters, crawler.Parameters); err != nil {
			l.Error("error getting crawler", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		schedule, err := searches.SetSchedule(c,
			clusterCtx.CS.BatchV1().CronJobs(clusterCtx.Namespace), clusterCtx.Name,
			req.CrawlerName, crawler, req.Schedule, req.Parameters)
		if err != nil {
			l.Error("error setting schedule", zap.Error(err))
			routes.WriteErr(c, err)
			return
		}

		routes.WriteSuccessResponse(c, schedule)
	}
}

func UnscheduleSearch() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "SetCrawler"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		id, err := searches.ParseRunID(c.Param("id"))
		if err != nil {
			routes.WriteErr(c, err)
			return
		}

		err = searches.RemoveSchedule(
			c,
			clusterCtx.CS.BatchV1().CronJobs(clusterCtx.Namespace),
			id,
		)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, nil)
	}
}

func ListScheduledSearches() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := zap.L().With(zap.String("endpoint", "GetCrawlerSchedules"))

		clusterCtx, exists := middleware.GetClusterContext(c)
		if !exists {
			l.Warn("cluster context not found")
			routes.WriteErrorResponse(c, http.StatusUnauthorized, schemas.ErrUnauthorized, nil)
			return
		}

		crawler, err := searches.ListSchedules(
			c,
			clusterCtx.CS.BatchV1().CronJobs(clusterCtx.Namespace),
		)
		if err != nil {
			routes.WriteErr(c, err)
			return
		}
		routes.WriteSuccessResponse(c, crawler)
	}
}
