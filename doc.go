// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package ocular is a package to provide the Ocular application,
// a code scanning orchestration tool for static application security testing.
// It is designed to have easily swappable components depending on: what you want to scan with,
// how you want to enumerate targets, and where you want to upload results to.
package ocular

/***************
 * Go Generate *
 ***************/

// The following go generate comments
// are used to enforce files that are automatically generated.
// This file should be the single source for go generate commands:

// generate the openAPI spec in YAML format (for better human readability)
//go:generate go run ./hack/generator/ -type open-api -output ./docs/swagger/openapi.yaml

// generate the openAPI spec in JSON format to be embedded in the application
// (JSON is more compact and easier for machines to parse)
//go:generate go run ./hack/generator/ -type open-api -format json -output ./pkg/api/static/openapi.json
