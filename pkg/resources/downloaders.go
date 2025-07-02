// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package downloaders provides the [Downloader] type which
// represents the init container that will download a static asset (target) to be scanned.
package resources

import (
	"context"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/storage"
)

// Downloader see the [schemas.Downloader] type for more details.
type Downloader schemas.Downloader

func (d Downloader) Validate(ctx context.Context, clusterCtx cluster.Context) error {
	return ValidateUserContainer(ctx, clusterCtx, schemas.UserContainer(d))
}

func (d Downloader) GetUserContainer() schemas.UserContainer {
	return schemas.UserContainer(d)
}

type DownloaderStorageBackend = storage.Backend[Downloader]

func NewDownloaderStorageBackend(clusterCtx cluster.Context) DownloaderStorageBackend {
	cm := storage.NewConfigMap[Downloader](
		config.State.Downloaders.ConfigMapName,
		clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace))
	return cm
}
