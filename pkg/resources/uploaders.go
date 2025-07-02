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

type Uploader schemas.UserContainerWithParameters

type UploaderStorageBackend = storage.Backend[Uploader]

func NewUploaderStorageBackend(clusterCtx cluster.Context) UploaderStorageBackend {
	cm := storage.NewConfigMap[Uploader](
		config.State.Uploaders.ConfigMapName,
		clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace))
	return cm
}

func (u Uploader) Validate(ctx context.Context, clusterCtx cluster.Context) error {
	return ValidateUserContainerWithParameters(
		ctx,
		clusterCtx,
		schemas.UserContainerWithParameters(u),
	)
}

func (u Uploader) GetUserContainerWithParameters() schemas.UserContainerWithParameters {
	return schemas.UserContainerWithParameters(u)
}
