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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/validators"
	"github.com/hashicorp/go-multierror"
)

// nolint:unused
// log is for logging in this package.
var clusteruploaderlog = logf.Log.WithName("clusteruploader-resource")

// SetupClusterUploaderWebhookWithManager registers the webhook for ClusterUploader in the manager.
func SetupClusterUploaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.ClusterUploader{}).
		WithValidator(&ClusterUploaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: this validator is currently only enabled for 'delete'.
// additional options can be specified in the 'verbs' parameter
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clusteruploader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clusteruploaders,verbs=delete,versions=v1beta1,name=vclusteruploader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ClusterUploaderCustomValidator struct is responsible for validating the ClusterUploader resource
// when it is created, updated, or deleted.
type ClusterUploaderCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateCreate(_ context.Context, obj *ocularcrashoverriderunv1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("cluster uploader validate create should not be registered, see NOTE in webhook/v1beta1/clusteruploader_webhook.go", "name", obj.GetName())
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *ocularcrashoverriderunv1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("cluster uploader validate update should not be registered, see NOTE in webhook/v1beta1/clusteruploader_webhook.go", "name", newObj.GetName())

	return nil, v.validateNewRequiredParameters(ctx, oldObj, newObj)
}

func (v *ClusterUploaderCustomValidator) validateNewRequiredParameters(ctx context.Context, oldUploader, newUploader *ocularcrashoverriderunv1beta1.ClusterUploader) error {
	newRequiredParameters := validators.GetNewRequiredParameters(oldUploader.Spec.Parameters, newUploader.Spec.Parameters)

	var paramErrors field.ErrorList
	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}

	for _, profile := range profiles.Items {

		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if uploaderRef.Name == newUploader.Name && uploaderRef.Kind == "ClusterUploader" {
				if !validators.AllParametersDefined(newRequiredParameters, uploaderRef.Parameters) {
					paramErrors = append(paramErrors, field.Required(field.NewPath("spec").Child("parameters"), fmt.Sprintf("uploader %s is still referenced by profile %s and not all new required parameters are defined", newUploader.Name, profile.Name)))
				}
			}
		}
	}

	if len(paramErrors) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "ClusterUploader"},
		newUploader.Name, paramErrors)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateDelete(ctx context.Context, obj *ocularcrashoverriderunv1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("validation for cluster uploader upon deletion", "name", obj.GetName())

	return nil, v.validateNoUploaderReferences(ctx, obj)
}

func (v *ClusterUploaderCustomValidator) validateNoUploaderReferences(ctx context.Context, uploader *ocularcrashoverriderunv1beta1.ClusterUploader) error {
	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	var allErrs *multierror.Error
	for _, profile := range profiles.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if uploaderRef.Name == uploader.Name && uploaderRef.Kind == "ClusterUploader" {
				allErrs = multierror.Append(allErrs,
					fmt.Errorf("this resource cannot be deleted because it is still referenced by 'Profile/%s in namespace %s'", profile.Name, profile.Namespace))
			}
		}
	}

	if allErrs.Len() == 0 {
		return nil
	}

	return apierrors.NewForbidden(
		schema.GroupResource{Group: "ocular.crashoverride.run", Resource: uploader.Name},
		uploader.Name, allErrs.ErrorOrNil())
}
