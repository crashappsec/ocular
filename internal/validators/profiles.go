// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package validators

import (
	"context"
	"errors"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/resources"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateProfile will validate the fields and references of the profile and return
// nil if the profile is valid. If the profile is not valid, a
// [k8s.io/apimachinery/pkg/api/errors.StatusError] is returned containing the
// details of the validation error.
func ValidateProfile(ctx context.Context, c client.Client, profile *v1beta1.Profile) error {
	var fieldErrors field.ErrorList

	volumeNames := make(map[string]struct{})
	for i, vol := range profile.Spec.Volumes {
		if _, exists := volumeNames[vol.Name]; exists {
			fieldErrors = append(fieldErrors, field.Duplicate(field.NewPath("spec").Child("volumes").Index(i).Child("name"), vol.Name))
		} else {
			volumeNames[vol.Name] = struct{}{}
		}
	}

	for i, uploaderRef := range profile.Spec.UploaderRefs {

		refField := field.NewPath("spec").Child("uploaderRefs").Index(i)
		uploader, err := resources.UploaderInvocationFromReference(ctx, c, profile.Namespace, uploaderRef)
		if refErr, ok := errors.AsType[resources.InvalidObjectReference](err); ok {
			fieldErrors = append(fieldErrors, field.Invalid(refField, uploaderRef, refErr.Message))
			continue
		} else if apierrors.IsNotFound(err) {
			fieldErrors = append(fieldErrors, field.NotFound(refField, uploaderRef))
			continue
		} else if err != nil {
			return err
		}

		fieldErrors = append(fieldErrors, ValidateParameterReference(ctx, refField, uploaderRef, uploader.Spec.Parameters)...)
		fieldErrors = append(fieldErrors, ValidateParentParameters(ctx, refField, uploaderRef, profile.Spec.Parameters)...)

		for i, vol := range uploader.Spec.Volumes {
			if _, exists := volumeNames[vol.Name]; exists {
				fieldErrors = append(fieldErrors, field.Duplicate(refField.Child("volumes").Index(i).Child("name"), vol.Name))
			} else {
				volumeNames[vol.Name] = struct{}{}
			}
		}
	}

	for i, c := range profile.Spec.Containers {
		fieldErrors = append(fieldErrors, ValidateContainerDefinition(ctx, field.NewPath("spec").Child("containers").Index(i), c.Container)...)
	}

	if len(fieldErrors) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Profile"}, profile.Name, fieldErrors)
}
