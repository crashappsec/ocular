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
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/identities"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/storage"
	"github.com/gin-gonic/gin"
)

// BackendConstructor is a function that takes a cluster context and returns a storage backend.
// It is used to create a storage backend for a specific type of object.
type BackendConstructor[T storage.Object] = func(clusterCtx cluster.Context) storage.Backend[T]

// storageRouteOpts holds the options for the storage route.
// It is used to enable or disable specific routes for the storage object.
type storageRouteOpts struct {
	DisableList   bool
	DisableGet    bool
	DisableSet    bool
	DisableDelete bool
}

// RequestBodyParser is a function that takes a gin context and returns an object of type T.
// It is used to parse the request body into the specified object type and allows for custom
// parsing methods.
type RequestBodyParser[T storage.Object] = func(c *gin.Context) (T, error)

// StorageRouteOption is a function that takes a storageRouteOpts and returns a modified storageRouteOpts.
type StorageRouteOption func(storageRouteOpts) storageRouteOpts

// WithDisableList is a StorageRouteOption that disables the list route.
func WithDisableList() StorageRouteOption {
	return func(opts storageRouteOpts) storageRouteOpts {
		opts.DisableList = true
		return opts
	}
}

// WithDisableGet is a StorageRouteOption that disables the get route.
func WithDisableGet() StorageRouteOption {
	return func(opts storageRouteOpts) storageRouteOpts {
		opts.DisableGet = true
		return opts
	}
}

// WithDisableSet is a StorageRouteOption that disables the set route.
func WithDisableSet() StorageRouteOption {
	return func(opts storageRouteOpts) storageRouteOpts {
		opts.DisableSet = true
		return opts
	}
}

// WithDisableDelete is a StorageRouteOption that disables the delete route.
func WithDisableDelete() StorageRouteOption {
	return func(opts storageRouteOpts) storageRouteOpts {
		opts.DisableDelete = true
		return opts
	}
}

// ContentTypeRequestBodyParser is a default request body parser that uses
// [gin.Context.ShouldBind] to parse the request body into the specified object type.
func ContentTypeRequestBodyParser[T any](c *gin.Context) (T, error) {
	var object T
	err := c.ShouldBind(&object)
	return object, err
}

// RegisterRoutesForStorageObject registers the routes for a storage object.
// It takes a gin router group, permission group, permission resource,
// backend constructor, request body parser, and options to enable or disable
// specific routes.
func RegisterRoutesForStorageObject[T storage.Object](
	g *gin.RouterGroup,
	permissionGroup, permissionResource string,
	backendConstructor BackendConstructor[T],
	bodyParser RequestBodyParser[T],
	opts ...StorageRouteOption,
) {
	var options storageRouteOpts
	for _, opt := range opts {
		options = opt(options)
	}

	if !options.DisableList {
		g.GET("",
			middleware.AuthorizeIdentity(
				identities.PermissionAuthorizer(
					identities.PermissionSet{
						Group:    permissionGroup,
						Resource: permissionResource,
						Verb:     "get",
					},
				),
			),
			func(c *gin.Context) {
				clusterCtx := middleware.MustClusterContext(c)
				backend := backendConstructor(clusterCtx)
				objects, err := backend.List(c)
				if err != nil {
					routes.WriteErr(c, err)
					return
				}
				routes.WriteSuccessResponse(c, objects)
			})
	}

	if !options.DisableGet {
		g.GET("/:name",
			middleware.AuthorizeIdentity(
				identities.PermissionAuthorizer(
					identities.PermissionSet{
						Group:    permissionGroup,
						Resource: permissionResource,
						Verb:     "get",
					},
				),
			),
			func(c *gin.Context) {
				clusterCtx := middleware.MustClusterContext(c)
				backend := backendConstructor(clusterCtx)
				name, exists := routes.MustParam(c, "name")
				if !exists {
					return
				}
				object, err := backend.Get(c, name)
				if err != nil {
					routes.WriteErr(c, err)
					return
				}
				routes.WriteSuccessResponse(c, object)
			})
	}

	if !options.DisableSet {
		g.POST("/:name",
			middleware.AuthorizeIdentity(
				identities.PermissionAuthorizer(
					identities.PermissionSet{
						Group:    permissionGroup,
						Resource: permissionResource,
						Verb:     "update",
					},
				),
			),
			func(c *gin.Context) {
				clusterCtx := middleware.MustClusterContext(c)
				backend := backendConstructor(clusterCtx)
				name, exists := routes.MustParam(c, "name")
				if !exists {
					return
				}

				object, err := bodyParser(c)
				if err != nil {
					routes.WriteErrorResponse(
						c,
						http.StatusBadRequest,
						schemas.ErrInvalidPayload,
						err.Error(),
					)
					return
				}

				if err = object.Validate(c, clusterCtx); err != nil {
					routes.WriteErrorResponse(
						c,
						http.StatusBadRequest,
						schemas.ErrInvalidPayload,
						err.Error(),
					)
					return
				}

				err = backend.Set(c, name, object)
				if err != nil {
					routes.WriteErr(c, err)
					return
				}
				routes.WriteSuccessResponse(c, nil)
			})
	}

	if !options.DisableDelete {
		g.DELETE("/:name",
			middleware.AuthorizeIdentity(
				identities.PermissionAuthorizer(
					identities.PermissionSet{
						Group:    permissionGroup,
						Resource: permissionResource,
						Verb:     "update", // this is update since were updating one field of the storage backend
					},
				),
			),
			func(c *gin.Context) {
				clusterCtx := middleware.MustClusterContext(c)
				backend := backendConstructor(clusterCtx)
				name, exists := routes.MustParam(c, "name")
				if !exists {
					return
				}

				err := backend.Delete(c, name)
				if err != nil {
					routes.WriteErr(c, err)
					return
				}
				routes.WriteSuccessResponse(c, nil)
			})
	}
}
