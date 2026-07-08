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

	corev1 "k8s.io/api/core/v1"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const MaxContainerNameLength = validationutils.DNS1035LabelMaxLength - 12 // "downloader-"

func ValidateContainerDefinition(ctx context.Context, fieldPath *field.Path, c corev1.Container) field.ErrorList {
	var fieldErrs field.ErrorList
	if len(c.Name) > MaxPipelineNameLength {
		fieldErrs = append(fieldErrs, field.Invalid(fieldPath.Child("name"), c.Name, "must be no more than 51 characters"))
	}

	if len(c.Command) == 0 {
		fieldErrs = append(fieldErrs, field.Invalid(fieldPath.Child("command"), c.Command, "container command must be specified, entrypoint from image config is not used"))
	}

	return fieldErrs
}
