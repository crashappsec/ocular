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
	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/storage"
)

type Profile schemas.Profile

func (p Profile) Validate(ctx context.Context, clusterCtx cluster.Context) error {
	for _, scanner := range p.Scanners {
		if err := ValidateUserContainer(ctx, clusterCtx, scanner); err != nil {
			return err
		}
	}

	allUploaders, err := NewUploaderStorageBackend(clusterCtx).List(ctx)
	if err != nil {
		return errs.New(
			errs.TypeUnknown,
			err,
			"failed to list uploaders",
		)
	}
	for _, uploadSpec := range p.Uploaders {
		uplr, exists := allUploaders[uploadSpec.Name]
		if !exists {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"uploader '%s' does not exist",
				uploadSpec.Name,
			)
		}

		if err := ValidateParameters(uploadSpec.Parameters, uplr.Parameters); err != nil {
			return err
		}
	}
	return nil
}

type ProfileStorageBackend = storage.Backend[Profile]

func NewProfileStorageBackend(clusterCtx cluster.Context) ProfileStorageBackend {
	cm := storage.NewConfigMap[Profile](
		config.State.Profiles.ConfigMapName,
		clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace))
	return cm
}
