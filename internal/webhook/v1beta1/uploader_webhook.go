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
	"strings"

	"github.com/crashappsec/ocular/internal/validators"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var uploaderlog = logf.Log.WithName("uploader-resource")

// SetupUploaderWebhookWithManager registers the webhook for Uploader in the manager.
func SetupUploaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.Uploader{}).
		WithValidator(&UploaderCustomValidator{
			c:      mgr.GetClient(),
			scheme: mgr.GetScheme(),
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
	c      client.Client
	scheme *runtime.Scheme
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateCreate(_ context.Context, uploader *v1beta1.Uploader) (admission.Warnings, error) {
	uploaderlog.Info("uploader validate create should not be registered, see NOTE in webhook/v1beta1/uploader_webhook.go", "name", uploader.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateUpdate(ctx context.Context, oldUploader, newUploader *v1beta1.Uploader) (admission.Warnings, error) {

	uploaderlog.Info("validating new parameters for uploader", "name", newUploader.GetName())

	newRequiredParams := parseNewRequiredParameters(oldUploader.Spec.Parameters, newUploader.Spec.Parameters)

	dependantProfiles, err := v.getDependantProfiles(ctx, oldUploader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	uploaderlog.Info("profiles", "profiles", len(dependantProfiles))
	for _, profile := range dependantProfiles {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if !refMatches(uploaderRef, oldUploader, "Uploader") {
				uploaderlog.Info("dodesn match", "match", false)
				continue
			}
			uploaderlog.Info("does match", "match", true)
			_, unset := validators.ParseSetParameters(uploaderRef, newRequiredParams)
			uploaderlog.Info("unset", "unset", unset)
			if len(unset) > 0 {
				var missingParamNames []string
				for _, u := range unset {
					missingParamNames = append(missingParamNames, u.Name)
				}
				return nil, apierrors.NewForbidden(
					schema.GroupResource{Group: "ocular.crashoverride.run", Resource: oldUploader.Name},
					oldUploader.Name, fmt.Errorf("dependant profile %s does not define newly required parameters: [%s]", profile.Name, strings.Join(missingParamNames, ",")))
			}
		}

	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateDelete(ctx context.Context, uploader *v1beta1.Uploader) (admission.Warnings, error) {
	uploaderlog.Info("validating no profile references deleted uploader", "name", uploader.GetName())

	dependantProfiles, err := v.getDependantProfiles(ctx, uploader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantProfiles) > 0 {
		profileNames := make([]string, 0, len(dependantProfiles))
		for _, profile := range dependantProfiles {
			profileNames = append(profileNames, profile.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "ocular.crashoverride.run", Resource: uploader.Name}, uploader.Name,
			fmt.Errorf("cannot delete uploader with dependant profiles: [%s]", strings.Join(profileNames, ",")))
	}

	return nil, nil
}

func (v *UploaderCustomValidator) getDependantProfiles(ctx context.Context, uploader *v1beta1.Uploader) ([]v1beta1.Profile, error) {
	var profilesInNamespace v1beta1.ProfileList
	if err := v.c.List(ctx, &profilesInNamespace, client.InNamespace(uploader.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list profiles in namespace %s: %w", uploader.Namespace, err)
	}

	var profiles []v1beta1.Profile
	for _, profile := range profilesInNamespace.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if refMatches(uploaderRef, uploader, "Uploader") {
				profiles = append(profiles, profile)
				break
			}
		}
	}

	return profiles, nil
}
