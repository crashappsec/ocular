// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package api

import (
	"io"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/api/middleware"
	"github.com/crashappsec/ocular/pkg/api/routes"
	routesV1 "github.com/crashappsec/ocular/pkg/api/routes/v1"
	"github.com/crashappsec/ocular/pkg/api/static"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/identities"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/gin-gonic/gin"
	batchV1 "k8s.io/api/batch/v1"
)

// InitializeEngine initializes the gin engine and sets up the routes.
// All routes (aside from [routes.Health]) are protected by authentication and authorization middleware.
// The engine is configured to use the [cluster.ContextManager] to assign the context
// to the request.
func InitializeEngine(ctxManager cluster.ContextManager) (*gin.Engine, error) {
	if config.IsEnvironmentIn(config.EnvProduction, config.EnvStaging) {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", routes.Health())
	router.GET("/version", routes.Version(config.Version, config.BuildTime, config.Commit))

	api := router.Group("/api")
	api.Use(
		// check context of cluster
		middleware.ContextAssigner(ctxManager),
		// authenticate the user with their bearer token
		middleware.BearerAuthenticator(),
		// if not authenticate with their client certificate
		middleware.CertificateAuthenticator(),
	)
	if config.IsEnvironmentIn(config.EnvDevelopment) {
		api.GET("/swagger", routes.ServeStatic(static.SwaggerHTML, "text/html"))
		api.GET(
			"/swagger/openapi.json",
			routes.ServeStatic(static.OpenAPISpec, "application/json"),
		)
	}

	v1 := api.Group("/v1")
	{
		routesV1.RegisterRoutesForStorageObject[resources.Uploader](
			v1.Group("/uploaders"),
			"", "configmaps",
			resources.NewUploaderStorageBackend,
			routesV1.ContentTypeRequestBodyParser,
		)

		routesV1.RegisterRoutesForStorageObject[resources.Downloader](
			v1.Group("/downloaders"),
			"", "configmaps",
			resources.NewDownloaderStorageBackend,
			routesV1.ContentTypeRequestBodyParser,
		)

		routesV1.RegisterRoutesForStorageObject[resources.Profile](
			v1.Group("/profiles"),
			"", "configmaps",
			resources.NewProfileStorageBackend,
			routesV1.ContentTypeRequestBodyParser,
		)

		routesV1.RegisterRoutesForStorageObject[resources.Crawler](
			v1.Group("/crawlers"),
			"", "configmaps",
			resources.NewCrawlerStorageBackend,
			routesV1.ContentTypeRequestBodyParser,
		)

		routesV1.RegisterRoutesForStorageObject[resources.Secret](
			v1.Group("/secrets"),
			"", "configmaps",
			resources.NewSecretStorageBackend,
			func(c *gin.Context) (resources.Secret, error) {
				secretVal, err := io.ReadAll(c.Request.Body)
				if err != nil {
					return resources.Secret{}, err
				}
				return secretVal, nil
			},
			routesV1.WithDisableGet(),
		)

		pipelines := v1.Group("/pipelines")
		{
			pipelines.GET("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "get",
						},
					),
				),
				routesV1.ListPipelines(),
			)

			pipelines.POST("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "create",
						},
						identities.PermissionSet{Resource: "configmap", Verb: "get"},
					),
					// additionally, if the user token is from the crawler audience, allow them to create pipelines
					identities.AudienceAuthorizer(identities.TokenAudienceCrawler),
				),
				routesV1.RunPipeline(),
			)

			pipelines.DELETE("/:id",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "delete",
						},
					),
				),
				routesV1.DeletePipeline(),
			)

			pipelines.GET("/:id",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "get",
						},
					),
				),
				routesV1.GetPipeline(),
			)
		}

		searches := v1.Group("/searches")
		{
			searches.GET("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "list",
						},
					),
				),
				routesV1.ListSearches(),
			)

			searches.GET("/:id",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "get",
						},
					),
				),
				routesV1.GetSearch(),
			)

			searches.POST("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{Resource: "configmaps", Verb: "get"},
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "create",
						},
					),
					// additionally, if the user token is from the crawler audience, allow them to run searches
					identities.AudienceAuthorizer(identities.TokenAudienceCrawler),
				),
				routesV1.RunSearch(),
			)

			searches.DELETE("/:id",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "jobs",
							Verb:     "delete",
						},
					),
				),
				routesV1.StopSearch(),
			)
		}

		scheduledSearches := v1.Group("/scheduled/searches")
		{
			scheduledSearches.GET("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "cronjobs",
							Verb:     "list",
						},
					),
				),
				routesV1.ListScheduledSearches(),
			)

			scheduledSearches.POST("",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "cronjobs",
							Verb:     "create",
						},
					),
				),
				routesV1.ScheduleSearch(),
			)

			scheduledSearches.DELETE("/:id",
				middleware.AuthorizeIdentity(
					identities.PermissionAuthorizer(
						identities.PermissionSet{
							Group:    batchV1.GroupName,
							Resource: "cronjobs",
							Verb:     "delete",
						},
					),
				),
				routesV1.UnscheduleSearch(),
			)
		}
	}

	return router, nil
}
