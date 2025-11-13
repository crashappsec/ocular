// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/internal/validators"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var profilelog = logf.Log.WithName("profile-resource")

// SetupProfileWebhookWithManager registers the webhook for Profile in the manager.
func SetupProfileWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Profile{}).
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

var _ webhook.CustomValidator = &ProfileCustomValidator{}

func (v *ProfileCustomValidator) validateProfile(ctx context.Context, profile *ocularcrashoverriderunv1beta1.Profile) error {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateUploaderReferences(ctx, v.c, profile)...)

	allErrs = append(allErrs, validators.ValidateAdditionalLabels(profile.Spec.AdditionalPodMetadata.Labels, field.NewPath("spec").Child("additionalPodMetadata").Child("labels"))...)

	allErrs = append(allErrs, validators.ValidateAdditionalAnnotations(profile.Spec.AdditionalPodMetadata.Annotations, field.NewPath("spec").Child("additionalPodMetadata").Child("annotations"))...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Profile"}, profile.Name, allErrs)
}

func validateUploaderReferences(ctx context.Context, c client.Client, profile *ocularcrashoverriderunv1beta1.Profile) field.ErrorList {
	var (
		allErrs     field.ErrorList
		volumeNames = map[string]struct{}{}
	)
	for _, uploaderRef := range profile.Spec.UploaderRefs {
		if uploaderRef.Namespace != "" && uploaderRef.Namespace != profile.Namespace {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("uploaderRefs").Child("namespace"), uploaderRef.Namespace, "must be empty or match the Profile namespace"))
			continue
		}
		var uploader ocularcrashoverriderunv1beta1.Uploader
		if err := c.Get(ctx, client.ObjectKey{Name: uploaderRef.Name, Namespace: profile.Namespace}, &uploader); err != nil {
			if apierrors.IsNotFound(err) {
				allErrs = append(allErrs, field.NotFound(field.NewPath("spec").Child("uploaderRefs").Child("name"), fmt.Sprintf("%s/%s", profile.Namespace, uploaderRef.Name)))
				continue
			}
			return field.ErrorList{field.InternalError(field.NewPath("spec").Child("uploaderRefs").Child("name"), fmt.Errorf("error fetching uploader %s/%s: %w", profile.Namespace, uploaderRef.Name, err))}
		}

		if paramErrs := validateSetParameters(uploaderRef.Name, field.NewPath("spec").Child("uploaderRefs").Child("parameters"), uploader.Spec.Parameters, uploaderRef.Parameters); len(paramErrs) > 0 {
			allErrs = append(allErrs, paramErrs...)
		}

		for _, vol := range uploader.Spec.Volumes {
			if _, exists := volumeNames[vol.Name]; exists {
				allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("uploaderRefs").Child("volumes").Child("name"), vol.Name))
			} else {
				volumeNames[vol.Name] = struct{}{}
			}
		}
	}

	return allErrs
}

func validateSetParameters(name string, fieldPath *field.Path, params []ocularcrashoverriderunv1beta1.ParameterDefinition, parameterSettings []ocularcrashoverriderunv1beta1.ParameterSetting) field.ErrorList {
	var allErrs field.ErrorList
	var setParams = map[string]struct{}{}
	for _, param := range parameterSettings {
		setParams[param.Name] = struct{}{}
	}
	for _, param := range params {
		if _, ok := setParams[param.Name]; !ok && param.Required {
			allErrs = append(allErrs, field.Required(fieldPath, fmt.Sprintf("parameter %s is required for uploader %s", param.Name, name)))
		}
	}
	return allErrs
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object but got %T", obj)
	}

	return nil, v.validateProfile(ctx, profile)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	profile, ok := newObj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object for the newObj but got %T", newObj)
	}
	profilelog.Info("Validation for Profile upon update", "name", profile.GetName())

	return nil, v.validateProfile(ctx, profile)
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
func (v *ProfileCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object but got %T", obj)
	}
	profilelog.Info("Validation for Profile upon deletion", "name", profile.GetName())

	return nil, v.checkForPipelinesReferencingProfile(ctx, profile)
}
