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
	"fmt"
	"path"

	"github.com/crashappsec/ocular/pkg/cluster"
	errs "github.com/crashappsec/ocular/pkg/errors"
	"github.com/crashappsec/ocular/pkg/schemas"
)

// It will check that the image is not empty, that the command is not empty,
func ValidateUserContainer(
	ctx context.Context,
	clusterCtx cluster.Context,
	ctnr schemas.UserContainer,
) error {
	for _, secret := range ctnr.Secrets {
		if !schemas.IsValidSecretMount(secret.MountType) {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"secret %s does not have a valid mount type: '%s'",
				secret.Name,
				secret.MountType,
			)
		}

		if secret.MountType == schemas.SecretMountTypeFile && !path.IsAbs(secret.MountTarget) {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"secret %s is a file mount, but a non absolute path was given: '%s'",
				secret.Name,
				secret.MountTarget,
			)
		}

		allSecrets, err := NewSecretStorageBackend(clusterCtx).List(ctx)
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		if secret.Required {
			_, exists := allSecrets[secret.Name]
			if !exists {
				return errs.New(
					errs.TypeBadRequest,
					nil,
					"secret %s does not exist but was marked required",
					secret.Name,
				)
			}
		}
	}
	return nil
}

// ValidateUserContainerWithParameters  checks that the UserContainerWithParameters is valid.
// It will check that the UserContainer is valid and that the parameters are valid.
// See [ValidateUserContainer] for more information on validating the UserContainer and
// [ValidateParameterDefinitions] for more information on validating the parameters.
func ValidateUserContainerWithParameters(
	ctx context.Context,
	clusterCtx cluster.Context,
	u schemas.UserContainerWithParameters,
) error {
	if err := ValidateUserContainer(ctx, clusterCtx, u.UserContainer); err != nil {
		return err
	}

	if err := ValidateParameterDefinitions(u.Parameters); err != nil {
		return err
	}
	return nil
}

// ValidateParameterDefinitions validates the parameter definitions and returns an [errs.Error]
// of type [errs.TypeBadRequest] if any of the parameters are invalid. It will first validate the
// name of each the parameter using [ValidateParameterName] and then check if the parameter
// has a default value if not required.
func ValidateParameterDefinitions(params map[string]schemas.ParameterDefinition) error {
	for name, param := range params {
		if err := ValidateParameterName(name); err != nil {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"parameter '%s' has invalid name: %v",
				name,
				err,
			)
		}

		if param.Required && param.Default != "" {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"parameter '%s' is required but has a default value",
				name,
			)
		}
	}
	return nil
}

// ValidateParameters checks that the parameters specified by a user are valid.
// It will check that the parameters are defined in the parameter definitions
// and that the required parameters are set.
func ValidateParameters(
	req map[string]string,
	parameters map[string]schemas.ParameterDefinition,
) error {
	// TODO(bryce): better parsing here so that we can match
	// case insensitive
	for name, param := range parameters {
		if _, set := req[schemas.FormatParamName(name)]; param.Required && !set {
			return errs.New(
				errs.TypeBadRequest,
				nil,
				"parameter '%s' is marked required but not set in request",
				name,
			)
		}
	}
	return nil
}

// ValidateParameterName checks that the parameter name is valid.
// It will check that the name is not empty, is not longer than 63 characters,
// and that it only contains alphanumeric characters, underscores, and dashes.
func ValidateParameterName(name string) error {
	if len(name) == 0 {
		return errs.New(errs.TypeBadRequest, nil, "parameter name cannot be empty")
	}
	if len(name) > 63 {
		return errs.New(
			errs.TypeBadRequest,
			nil,
			"parameter name cannot be longer than 63 characters",
		)
	}
	for _, r := range name {
		validCharacter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '_' ||
			r == '-'
		if !validCharacter {
			return errs.New(errs.TypeBadRequest, nil, "invalid character in parameter name")
		}
	}
	return nil
}
