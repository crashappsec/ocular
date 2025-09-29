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
var uploaderlog = logf.Log.WithName("uploader-resource")

// SetupUploaderWebhookWithManager registers the webhook for Uploader in the manager.
func SetupUploaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Uploader{}).
		WithValidator(&UploaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: currently the uploader is only configured to run as a validating webhook
// during the update and/or deletion of a Uploader resource to validate that
// 1) no new required parameters have been added that are not defined in
//    existing Pipelines resources that reference it (on update), and
// 2) no Pipeline resources referring to it exist (on delete).
// Creation is currently not needed since most of the work is handled by the
// k8s OpenAPI schema validation. If in the future there is a need to validate
// Uploader resources on creation, the ValidateCreate method below can be implemented and 'create'
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-uploader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=uploaders,verbs=delete;update,versions=v1beta1,name=vuploader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// UploaderCustomValidator struct is responsible for validating the Uploader resource
// when it is created, updated, or deleted.
type UploaderCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &UploaderCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	uploader, ok := obj.(*ocularcrashoverriderunv1beta1.Uploader)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object but got %T", obj)
	}

	uploaderlog.Info("uploader validate update should not be registered, see NOTE in webhook/v1beta1/uploader_webhook.go", "name", uploader.GetName())

	return nil, nil
}

func (v *UploaderCustomValidator) validateNewRequiredParameters(ctx context.Context, oldUploader, newUploader *ocularcrashoverriderunv1beta1.Uploader) error {
	newRequiredParameters := validators.GetNewRequiredParameters(oldUploader.Spec.Parameters, newUploader.Spec.Parameters)

	var paramErrors field.ErrorList
	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}

	for _, profile := range profiles.Items {

		for _, uploaderRef := range profile.Spec.UploaderRefs {
			var namespace = uploaderRef.Namespace
			if namespace == "" {
				namespace = profile.Namespace
			}
			if uploaderRef.Name == newUploader.Name && namespace == newUploader.Namespace {
				if !validators.AllParametersDefined(newRequiredParameters, uploaderRef.Parameters) {
					paramErrors = append(paramErrors, field.Required(field.NewPath("spec").Child("parameters"), fmt.Sprintf("uplodaer %s is still referenced by profile %s and not all new required parameters are defined", newUploader.Name, profile.Name)))
				}
			}
		}
	}

	if len(paramErrors) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Uploader"},
		newUploader.Name, paramErrors)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	uploader, ok := newObj.(*ocularcrashoverriderunv1beta1.Uploader)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object for the newObj but got %T", newObj)
	}

	oldUploader, ok := oldObj.(*ocularcrashoverriderunv1beta1.Uploader)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object for the oldObj but got %T", oldObj)
	}

	uploaderlog.Info("validating new parameters for uploader", "name", uploader.GetName())

	return nil, v.validateNewRequiredParameters(ctx, oldUploader, uploader)
}

func (v *UploaderCustomValidator) validateNoUploaderReferences(ctx context.Context, uploader *ocularcrashoverriderunv1beta1.Uploader) error {
	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	var allErrs field.ErrorList
	for _, profile := range profiles.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			var namespace = uploaderRef.Namespace
			if namespace == "" {
				namespace = profile.Namespace
			}
			if uploaderRef.Name == uploader.Name && namespace == uploader.Namespace {
				uploaderlog.Info("found profile reference to uploader", "profile", profile.GetName(), "name", uploader.GetName())
				allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), uploader.Name, "cannot be deleted because it is still referenced by a Profile resource"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Uploader"},
		uploader.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	uploader, ok := obj.(*ocularcrashoverriderunv1beta1.Uploader)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object but got %T", obj)
	}

	uploaderlog.Info("validating no profile references deleted uploader", "name", uploader.GetName())

	return nil, v.validateNoUploaderReferences(ctx, uploader)
}
