// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/crashappsec/ocular/internal/utilities"
	"github.com/crashappsec/ocular/pkg/api"
	"github.com/crashappsec/ocular/pkg/pipelines"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-multierror"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
	"go.uber.org/zap"
)

const (
	TypeProfile                = "Profile"
	TypeUserContainer          = "UserContainer"
	TypeCrawler                = "Crawler"
	TypeDownloader             = "Downloader"
	TypeUploader               = "Uploader"
	TypePipelineRequest        = "PipelineRequest"
	TypePipeline               = "Pipeline"
	TypeSecret                 = "Secret"
	TypeSearchRequest          = "SearchRequest"
	TypeSearch                 = "Search"
	TypeScheduledSearchRequest = "ScheduledSearchRequest"
	TypeScheduledSearch        = "ScheduledSearch"
	TypeVersionResponse        = "APIVersion"
)

var AllTypes = map[string]any{
	TypeProfile:                resources.Profile{},
	TypeUserContainer:          schemas.UserContainer{},
	TypeCrawler:                resources.Crawler{},
	TypeDownloader:             resources.Downloader{},
	TypeUploader:               resources.Uploader{},
	TypePipeline:               pipelines.Pipeline{},
	TypePipelineRequest:        schemas.PipelineRequest{},
	TypeSecret:                 nil,
	TypeSearch:                 schemas.Search{},
	TypeScheduledSearch:        schemas.ScheduledSearch{},
	TypeSearchRequest:          schemas.SearchRequest{},
	TypeScheduledSearchRequest: schemas.ScheduleRequest{},
	TypeVersionResponse:        schemas.APIVersionResponse{},
}

var IgnoredRoutes = map[RouteInfo]struct{}{
	{"/api/swagger", "GET"}:              {},
	{"/api/swagger/openapi.json", "GET"}: {},
}

var Routes = utilities.MergeMaps(map[RouteInfo]RouteTypes{
	{"/health", "GET"}: {
		Description:     "Health check endpoint",
		DisableSecurity: true,
		Tags:            []string{"system"},
	},
	{"/version", "GET"}: {
		Description:     "Health check endpoint",
		DisableSecurity: true,
		Tags:            []string{"system"},
		Response:        TypeVersionResponse,
	},
	{"/api/v1/pipelines", "POST"}: {
		Request: TypePipelineRequest,
		// Parameters:  schemas.PipelineRequest{},
		Response:    TypePipeline,
		Description: "Starts a new pipeline with the given request",
		Tags:        []string{"pipelines"},
	},
	{"/api/v1/pipelines/{id}", "GET"}: {
		Response: TypePipeline,
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Description: "Gets a pipeline by ID",
		Tags:        []string{"pipelines"},
	},
	{"/api/v1/pipelines/{id}", "DELETE"}: {
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Description: "Terminates a pipeline execution by ID, if it is running or pending",
		Tags:        []string{"pipelines"},
	},
	{"/api/v1/pipelines", "GET"}: {
		ResponseList: TypePipeline,
		Description:  "Gets all pipelines",
		Tags:         []string{"pipelines"},
	},
	{"/api/v1/searches", "POST"}: {
		Request:     TypeSearchRequest,
		Response:    TypeSearch,
		Description: "Starts a new search with the given request",
		Tags:        []string{"searches"},
	},
	{"/api/v1/searches/{id}", "GET"}: {
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Response:    TypeSearch,
		Description: "Gets a search by ID",
	},
	{"/api/v1/searches/{id}", "DELETE"}: {
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Description: "Terminates a search execution by ID, if it is running or pending",
		Tags:        []string{"searches"},
	},
	{"/api/v1/searches", "GET"}: {
		ResponseList: TypeSearch,
		Description:  "Gets all searches",
		Tags:         []string{"searches"},
	},
	{"/api/v1/scheduled/searches", "GET"}: {
		Response:    TypeScheduledSearch,
		Description: "Gets all scheduled searches",
		Tags:        []string{"scheduled searches"},
	},
	{"/api/v1/scheduled/searches", "POST"}: {
		Request:     TypeScheduledSearchRequest,
		Response:    TypeScheduledSearch,
		Description: "Schedules a new search to run at the given cron schedule",
		Tags:        []string{"scheduled searches"},
	},
	{"/api/v1/scheduled/searches/{id}", "GET"}: {
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Response:    TypeScheduledSearch,
		Description: "Gets a scheduled search by ID",
	},
	{"/api/v1/scheduled/searches/{id}", "DELETE"}: {
		Parameters: struct {
			ID string `path:"id" json:"id" yaml:"id"`
		}{},
		Description: "Removes a scheduled search by ID, if it exists",
	},
}, GenerateResourcesMap(
	ResourceRoute{Name: "profiles", Type: TypeProfile},
	ResourceRoute{Name: "crawlers", Type: TypeCrawler},
	ResourceRoute{Name: "downloaders", Type: TypeDownloader},
	ResourceRoute{Name: "uploaders", Type: TypeUploader},
	ResourceRoute{Name: "secrets", Type: TypeSecret},
))

type ResourceRoute struct {
	Name string
	Type string
}

func GenerateResourcesMap(resources ...ResourceRoute) map[RouteInfo]RouteTypes {
	m := map[RouteInfo]RouteTypes{}
	for _, r := range resources {
		m[RouteInfo{Path: fmt.Sprintf("/api/v1/%s", r.Name), Method: "GET"}] = RouteTypes{
			ResponseList: r.Type,
			Description:  fmt.Sprintf("Get all %s", r.Name),
			Tags:         []string{r.Name},
		}
		m[RouteInfo{Path: fmt.Sprintf("/api/v1/%s/{name}", r.Name), Method: "POST"}] = RouteTypes{
			Request: r.Type,
			Parameters: struct {
				Name string `path:"name" json:"name" yaml:"name"`
			}{},
			Description: fmt.Sprintf("Creates or updates the %s with name {name}", r.Name),
			Tags:        []string{r.Name},
		}
		m[RouteInfo{Path: fmt.Sprintf("/api/v1/%s/{name}", r.Name), Method: "GET"}] = RouteTypes{
			Response: r.Type,
			Parameters: struct {
				Name string `path:"name" json:"name" yaml:"name"`
			}{},
			Tags:        []string{r.Name},
			Description: fmt.Sprintf("Get %s by {name}", r.Name),
		}
		m[RouteInfo{Path: fmt.Sprintf("/api/v1/%s/{name}", r.Name), Method: "DELETE"}] = RouteTypes{
			Request: r.Type,
			Parameters: struct {
				Name string `path:"name" json:"name" yaml:"name"`
			}{},
			Tags:        []string{r.Name},
			Description: fmt.Sprintf("Removes a %s by name, if it exists", r.Name),
		}
	}
	return m
}

type RouteInfo struct {
	Path   string
	Method string
}

func (r RouteInfo) String() string {
	return fmt.Sprintf("%s %s", r.Method, r.Path)
}

type RouteTypes struct {
	Request         string
	Response        string
	ResponseList    string
	Parameters      any
	DisableSecurity bool
	Tags            []string
	Description     string
}

const SecurityName = "BearerAuth"

// This library is dumb and for some reason requires a struct to be defined
// and have a field in order to use it as a parameter type in OpenAPI.
type NoParams struct {
	NoParams bool `json:"NoParams" yaml:"NoParams"`
}

func WriteOpenAPI(w io.Writer, format string) error {
	l := zap.L()

	/*********************
	 * API Wide settings *
	 *********************/

	reflector := openapi3.Reflector{}
	reflector.Spec = &openapi3.Spec{Openapi: "3.0.3"}
	reflector.Spec.Info.
		WithTitle("Ocular API").
		WithVersion("0.0.0-beta.3").
		WithDescription("API to code scanning orchestration engine for Ocular")

	l.Debug("building api routes")
	gin.SetMode(gin.ReleaseMode) // Set Gin to release mode to avoid debug logs
	engine, err := api.InitializeEngine(nil)
	if err != nil {
		return fmt.Errorf("failed to initialize API engine: %w", err)
	}

	obj := openapi3.SchemaTypeObject
	boolean := openapi3.SchemaTypeBoolean
	array := openapi3.SchemaTypeArray
	str := openapi3.SchemaTypeString

	l.Debug("ensuring all routes have types defined")
	var merr *multierror.Error

	if reflector.Spec.Components == nil {
		reflector.Spec.WithComponents(openapi3.Components{})
	}

	if reflector.Spec.Components.Schemas == nil {
		reflector.Spec.Components.WithSchemas(openapi3.ComponentsSchemas{})
	}

	if reflector.Spec.Components.Responses == nil {
		reflector.Spec.Components.WithResponses(openapi3.ComponentsResponses{})
	}
	if reflector.Spec.Components.SecuritySchemes == nil {
		reflector.Spec.Components.WithSecuritySchemes(openapi3.ComponentsSecuritySchemes{})
	}
	/************************
	 * API Security Schemes *
	 ************************/

	reflector.Spec.SetHTTPBearerTokenSecurity(
		SecurityName, "jwt", "Bearer authentication using JWT tokens")

	/*****************************
	 * API Error Response Schema *
	 *****************************/

	description := "Error response from API"
	reflector.Spec.Components.Responses.WithMapOfResponseOrRefValuesItem("ErrorResponse",
		openapi3.ResponseOrRef{
			Response: &openapi3.Response{
				Description: "Error response",
				Content: map[string]openapi3.MediaType{
					"application/json": {
						Schema: &openapi3.SchemaOrRef{
							Schema: &openapi3.Schema{
								Type: &obj,
								Properties: map[string]openapi3.SchemaOrRef{
									"success": {
										Schema: &openapi3.Schema{
											Type: &boolean,
											Enum: []interface{}{
												false,
											},
										},
									},
									"error": {
										Schema: &openapi3.Schema{
											Type:        &str,
											Description: &description,
										},
									},
								},
							},
						},
					},
					"application/x-yaml": {
						Schema: &openapi3.SchemaOrRef{
							Schema: &openapi3.Schema{
								Type: &obj,
								Properties: map[string]openapi3.SchemaOrRef{
									"success": {
										Schema: &openapi3.Schema{
											Type: &boolean,
											Enum: []interface{}{
												false,
											},
										},
									},
									"error": {
										Schema: &openapi3.Schema{
											Type:        &str,
											Description: &description,
										},
									},
								},
							},
						},
					},
				},
			},
		})

	/********************
	 * API Type Schemas *
	 ********************/

	for tName, t := range AllTypes {
		allSchemas := map[string]jsonschema.Schema{}
		propertySchema, err := reflector.Reflect(
			t,
			jsonschema.CollectDefinitions(func(name string, schema jsonschema.Schema) {
				allSchemas[name] = schema
			}),
			jsonschema.DefinitionsPrefix("#/components/schemas/"),
		)
		allSchemas[tName] = propertySchema
		if err != nil {
			merr = multierror.Append(
				merr,
				fmt.Errorf("failed to reflect type '%s': %w", tName, err),
			)
			continue
		}
		if reflector.Spec.Components.Schemas == nil {
			reflector.Spec.Components.WithSchemas(openapi3.ComponentsSchemas{})
		}

		for name, schemaName := range allSchemas {
			schemaRef := openapi3.SchemaOrRef{}
			schemaRef.FromJSONSchema(schemaName.ToSchemaOrBool())
			reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
				name,
				schemaRef,
			)
		}
	}

	if merr.ErrorOrNil() != nil {
		return fmt.Errorf("failed to reflect types: %w", merr)
	}
	l.Debug("iterating over engine routes to build OpenAPI spec")
	merr = nil

	/**************
	 * API Routes *
	 **************/

	for _, r := range engine.Routes() {
		rInfo := RouteInfo{Path: pathParamsToOpenAPI(r.Path), Method: r.Method}
		route, exists := Routes[rInfo]
		if _, ignore := IgnoredRoutes[rInfo]; !exists && !ignore {
			merr = multierror.Append(
				merr,
				fmt.Errorf("route '%s' is not defined", rInfo),
			)
			continue
		} else if ignore {
			continue
		}

		method, err := reflector.NewOperationContext(strings.ToLower(rInfo.Method), rInfo.Path)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		if !route.DisableSecurity {
			method.AddSecurity(SecurityName)
		}

		method.SetDescription(route.Description)
		method.SetTags(route.Tags...)

		var requestOptions []openapi.ContentOption
		if route.Request != "" {
			requestOptions = append(
				requestOptions,
				openapi.WithCustomize(func(cor openapi.ContentOrReference) {
					reqBody, ok := cor.(*openapi3.RequestBodyOrRef)
					if !ok {
						l.With(zap.String("path", r.Path), zap.String("method", r.Method)).
							Error("unexpected type for request body, expected RequestBodyOrRef")
						return
					}
					reqBody.RequestBody = &openapi3.RequestBody{
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: "#/components/schemas/" + route.Request,
									},
								},
							},
							"application/x-yaml": {
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: "#/components/schemas/" + route.Request,
									},
								},
							},
						},
					}
				}),
				openapi.WithContentType("application/json"),
			)
		}

		params := route.Parameters
		if params == nil {
			params = NoParams{}
		}

		method.AddReqStructure(
			params,
			requestOptions...,
		)

		method.AddRespStructure(
			new(struct{}),
			openapi.WithHTTPStatus(200),
			openapi.WithCustomize(func(cor openapi.ContentOrReference) {
				reqBody, ok := cor.(*openapi3.ResponseOrRef)
				if !ok {
					l.With(zap.String("path", r.Path), zap.String("method", r.Method)).
						Error("unexpected type for request body, expected ResponseOrRef")
					return
				}

				properties := map[string]openapi3.SchemaOrRef{
					"success": {
						Schema: &openapi3.Schema{
							Type: &boolean,
							Enum: []interface{}{
								true,
							},
						},
					},
				}
				if route.Response != "" {
					properties["response"] = openapi3.SchemaOrRef{
						SchemaReference: &openapi3.SchemaReference{
							Ref: "#/components/schemas/" + route.Response,
						},
					}
				} else if route.ResponseList != "" {
					properties["response"] = openapi3.SchemaOrRef{
						Schema: &openapi3.Schema{
							Type: &array,
							Items: &openapi3.SchemaOrRef{
								SchemaReference: &openapi3.SchemaReference{
									Ref: "#/components/schemas/" + route.ResponseList,
								},
							},
						},
					}
				}

				reqBody.Response = &openapi3.Response{
					Content: map[string]openapi3.MediaType{
						"application/json": {
							Schema: &openapi3.SchemaOrRef{
								Schema: &openapi3.Schema{
									Type:       &obj,
									Properties: properties,
								},
							},
						},
						"application/x-yaml": {
							Schema: &openapi3.SchemaOrRef{
								Schema: &openapi3.Schema{
									Type:       &obj,
									Properties: properties,
								},
							},
						},
					},
				}
			}))

		for _, respCode := range []int{4, 5} {
			method.AddRespStructure(
				new(struct{}),
				openapi.WithHTTPStatus(respCode),
				openapi.WithCustomize(func(cor openapi.ContentOrReference) {
					reqBody, ok := cor.(*openapi3.ResponseOrRef)
					if !ok {
						l.With(zap.String("path", r.Path), zap.String("method", r.Method)).
							Error("unexpected type for request body, expected RequestBodyOrRef")
						return
					}
					reqBody.SetReference("#/components/responses/ErrorResponse")
				}),
			)
		}
		method.AddReqStructure(route.Request)

		if err = reflector.AddOperation(method); err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
	}

	if merr != nil {
		return fmt.Errorf("failed to build API routes: %w", merr)
	}

	var spec []byte
	switch format {
	case "yaml":
		spec, err = reflector.Spec.MarshalYAML()
	case "json":
		spec, err = json.MarshalIndent(reflector.Spec, "", "  ")
	default:
		err = fmt.Errorf("unsupported format '%s', must be 'yaml' or 'json'", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAPI spec: %w", err)
	}

	if _, err = w.Write(spec); err != nil {
		return fmt.Errorf("failed to write OpenAPI spec: %w", err)
	}
	return nil
}

var pathParamRegex = regexp.MustCompile(`/:([a-zA-Z0-9_]+)`)

func pathParamsToOpenAPI(ginPath string) string {
	// Convert Gin-style path parameters (e.g., /:id) to OpenAPI-style (e.g., {id})
	pathParams := pathParamRegex.ReplaceAllString(ginPath, `/{${1}}`)

	return pathParams
}
