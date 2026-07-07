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
	"fmt"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/containers"
	"github.com/crashappsec/ocular/internal/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const MaxPipelineNameLength = validationutils.DNS1035LabelMaxLength - 9 // "pipeline-"

func ValidatePipeline(ctx context.Context, c client.Client, pipeline *v1beta1.Pipeline) error {
	var fieldErrs field.ErrorList
	if len(pipeline.Name) > MaxPipelineNameLength {
		fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("metadata").Child("name"), pipeline.Name, "must be no more than 52 characters"))
	}

	pipelineVolumes := make(map[string]struct{})

	// validate profile
	var refErr resources.InvalidObjectReference
	profile, err := resources.ProfileInvocationFromReference(ctx, c, pipeline.Namespace, pipeline.Spec.ProfileRef)
	if errors.As(err, &refErr) {
		fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("profileRef"), pipeline.Spec.ProfileRef, refErr.Message))
	} else if apierrors.IsNotFound(err) {
		fieldErrs = append(fieldErrs, field.NotFound(
			field.NewPath("spec").Child("profileRef"),
			pipeline.Spec.ProfileRef))
	} else if err != nil {
		return apierrors.NewInternalError(err)
	}

	fieldErrs = append(fieldErrs, ValidateParameterReference(ctx,
		field.NewPath("spec").Child("profileRef"),
		pipeline.Spec.ProfileRef, profile.Spec.Parameters)...)
	fieldErrs = append(fieldErrs, ValidateNoParentParameters(field.NewPath("spec").Child("profileRef"), pipeline.Spec.ProfileRef)...)

	scanners := containers.FilterConditionalContainers(profile.Spec.Containers, profile.Spec.Parameters, pipeline.Spec.ProfileRef.Parameters)
	if len(scanners) == 0 {
		fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("profileRef"),
			pipeline.Spec.ProfileRef, "No scanners were included, ensure that at least one scanner's `includeIf` is true"))
	}

	for _, vol := range profile.Spec.Volumes {
		pipelineVolumes[vol.Name] = struct{}{}
	}

	// validate downloader
	downloader, err := resources.DownloaderInvocationFromReference(ctx, c, pipeline.Namespace, pipeline.Spec.DownloaderRef)
	if errors.As(err, &refErr) {
		fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("downloaderRef"), pipeline.Spec.DownloaderRef, refErr.Message))
	} else if apierrors.IsNotFound(err) {
		fieldErrs = append(fieldErrs, field.Invalid(field.NewPath("spec").Child("downloaderRef"), pipeline.Spec.DownloaderRef, "referenced downloader could not be found"))
	} else if err != nil {
		return apierrors.NewInternalError(err)
	}

	fieldErrs = append(fieldErrs, ValidateParameterReference(ctx,
		field.NewPath("spec").Child("downloaderRef"),
		pipeline.Spec.DownloaderRef, downloader.Spec.Parameters)...)
	fieldErrs = append(fieldErrs, ValidateNoParentParameters(field.NewPath("spec").Child("downloaderRef"), pipeline.Spec.DownloaderRef)...)

	// validate no conflicting volumes
	for _, vol := range downloader.Spec.Volumes {
		if _, exists := pipelineVolumes[vol.Name]; exists {
			fieldErrs = append(fieldErrs, field.Invalid(
				field.NewPath("spec").Child("downloaderRef"),
				pipeline.Spec.DownloaderRef, fmt.Sprintf("downloader volume %s conflicts with profile volume with the same name", vol.Name)))
		}
	}

	// validate service accounts
	var serviceAccount corev1.ServiceAccount
	err = c.Get(ctx, client.ObjectKey{Name: pipeline.Spec.ServiceAccountName, Namespace: pipeline.Namespace}, &serviceAccount)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return apierrors.NewInternalError(err)
		}
		fieldErrs = append(fieldErrs, field.NotFound(field.NewPath("spec").Child("serviceAccountName"), pipeline.Spec.ServiceAccountName))
	}

	if len(fieldErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Pipeline"},
		pipeline.Name, fieldErrs)
}
