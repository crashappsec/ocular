// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package config

// Version, BuildTime, Commit are set during the build process.
// They can be set using the -ldflags option in the go build command.
// For example: go build -ldflags "-X 'github.com/crashappsec/ocular/internal/config.Version=1.0.0' -X 'github.com/crashappsec/ocular/internal/config.BuildTime=$(date +%Y-%m-%d)' -X 'github.com/crashappsec/ocular/internal/config.Commit=$(git rev-parse HEAD)'"

var (
	// Version is the version of the application.
	Version = "dev"
	// BuildTime is the date when the application was built.
	BuildTime = "unknown"
	// Commit is the commit hash of the application.
	Commit = "unknown"
)
