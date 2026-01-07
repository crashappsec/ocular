// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package validators

import (
	"fmt"
	"strings"

	"github.com/crashappsec/ocular/api/v1beta1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateAdditionalLabels(labels map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := v1validation.ValidateLabels(labels, fldPath)
	for key := range labels {
		if strings.HasPrefix(key, v1beta1.Group) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(key), key, fmt.Sprintf("additional labels cannot use reserved prefix: %s", v1beta1.Group)))
		}
	}
	return allErrs
}

func ValidateAdditionalAnnotations(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	fieldErrors := apimachineryvalidation.ValidateAnnotations(annotations, fldPath)
	for key := range annotations {
		if strings.HasPrefix(key, v1beta1.Group) {
			fieldErrors = append(fieldErrors, field.Invalid(fldPath.Child(key), key, fmt.Sprintf("additional annotations cannot use reserved prefix: %s", v1beta1.Group)))
		}
	}

	return fieldErrors
}
