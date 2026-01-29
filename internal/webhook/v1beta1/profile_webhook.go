// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"github.com/crashappsec/ocular/internal/resources"
	"github.com/crashappsec/ocular/internal/validators"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var profilelog = logf.Log.WithName("profile-resource")

// SetupProfileWebhookWithManager registers the webhook for Profile in the manager.
func SetupProfileWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.Profile{}).
		WithValidator(&ProfileCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-profile,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=profiles,verbs=create;update;delete,versions=v1beta1,name=vprofile-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ProfileCustomValidator struct is responsible for validating the Profile resource
// when it is created, updated, or deleted.
type ProfileCustomValidator struct {
	c client.Client
}

func (v *ProfileCustomValidator) validateProfile(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) error {
	var allErrs field.ErrorList

	if fieldErrs, err := v.validateUploaderReferences(ctx, profile); err != nil {
		return err
	} else {
		allErrs = append(allErrs, fieldErrs...)
	}

	allErrs = append(allErrs, validators.ValidateAdditionalLabels(profile.Spec.AdditionalPodMetadata.Labels,
		field.NewPath("spec").Child("additionalPodMetadata").Child("labels"))...)

	allErrs = append(allErrs, validators.ValidateAdditionalAnnotations(profile.Spec.AdditionalPodMetadata.Annotations,
		field.NewPath("spec").Child("additionalPodMetadata").Child("annotations"))...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Profile"}, profile.Name, allErrs)
}

func (v *ProfileCustomValidator) validateUploaderReferences(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) (field.ErrorList, error) {
	var (
		allErrs     field.ErrorList
		volumeNames = map[string]struct{}{}
	)
	for i, uploaderRef := range profile.Spec.UploaderRefs {
		var (
			refErr   resources.InvalidObjectReference
			refField = field.NewPath("spec").Child("uploaderRefs").Index(i)
		)

		uploaderSpec, err := resources.UploaderSpecFromReference(ctx, v.c, profile.Namespace, uploaderRef.ObjectReference)
		if errors.As(err, &refErr) {
			allErrs = append(allErrs, field.Invalid(refField, uploaderRef, refErr.Message))
			continue
		} else if apierrors.IsNotFound(err) {
			allErrs = append(allErrs, field.Invalid(refField, uploaderRef, "referenced uploader could not be found"))
			continue
		} else if err != nil {
			return allErrs, err
		}

		if paramErrs := validateSetParameters(uploaderRef.Name, refField.Child("parameters"), uploaderSpec.Parameters, uploaderRef.Parameters); len(paramErrs) > 0 {
			allErrs = append(allErrs, paramErrs...)
		}

		for _, vol := range uploaderSpec.Volumes {
			if _, exists := volumeNames[vol.Name]; exists {
				allErrs = append(allErrs, field.Duplicate(refField.Child("volumes").Child("name"), vol.Name))
			} else {
				volumeNames[vol.Name] = struct{}{}
			}
		}
	}

	return allErrs, nil
}

func validateSetParameters(name string, fieldPath *field.Path, params []ocularcrashoverriderunv1beta1.ParameterDefinition, parameterSettings []ocularcrashoverriderunv1beta1.ParameterSetting) field.ErrorList {
	var allErrs field.ErrorList
	var setParams = map[string]struct{}{}
	for _, param := range parameterSettings {
		setParams[param.Name] = struct{}{}
	}
	for _, param := range params {
		if _, ok := setParams[param.Name]; !ok && param.Required {
			allErrs = append(allErrs, field.Required(fieldPath, fmt.Sprintf("parameter %s is required for %s", param.Name, name)))
		}
	}
	return allErrs
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateCreate(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) (admission.Warnings, error) {

	return nil, v.validateProfile(ctx, profile)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateUpdate(ctx context.Context, _, newProfile *ocularcrashoverriderunv1beta1.Profile) (admission.Warnings, error) {

	return nil, v.validateProfile(ctx, newProfile)
}

func (v *ProfileCustomValidator) checkForPipelinesReferencingProfile(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) error {
	var pipelines ocularcrashoverriderunv1beta1.PipelineList
	if err := v.c.List(ctx, &pipelines, client.InNamespace(profile.Namespace)); err != nil {
		return fmt.Errorf("failed to list pipelines: %w", err)
	}

	var allErrs field.ErrorList

	for _, pipeline := range pipelines.Items {
		namespace := pipeline.Spec.ProfileRef.Namespace
		if namespace == "" {
			namespace = pipeline.Namespace
		}
		if pipeline.Spec.ProfileRef.Name == profile.Name && namespace == profile.Namespace {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), profile.Name, "cannot be deleted because it is still referenced by a Pipeline resource"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Profile"},
		profile.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateDelete(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) (admission.Warnings, error) {
	profilelog.Info("Validation for Profile upon deletion", "name", profile.GetName())

	return nil, v.checkForPipelinesReferencingProfile(ctx, profile)
}
