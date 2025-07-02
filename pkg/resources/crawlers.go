// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"context"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/storage"
)

// Crawler see the [schemas.Crawler] type for more details.
type Crawler schemas.Crawler

func (c Crawler) Validate(ctx context.Context, clusterCtx cluster.Context) error {
	return ValidateUserContainerWithParameters(ctx, clusterCtx,
		schemas.UserContainerWithParameters(c))
}

func (c Crawler) GetUserContainerWithParameters() schemas.UserContainerWithParameters {
	return schemas.UserContainerWithParameters(c)
}

// CrawlerStorageBackend is the type of the storage backend used to store
// [Crawler] configurations.
type CrawlerStorageBackend = storage.Backend[Crawler]

// NewCrawlerStorageBackend creates a new [CrawlerStorageBackend] for the
// given [cluster.Context].
func NewCrawlerStorageBackend(clusterCtx cluster.Context) CrawlerStorageBackend {
	cm := storage.NewConfigMap[Crawler](
		config.State.Crawlers.ConfigMapName,
		clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace))
	return cm
}
