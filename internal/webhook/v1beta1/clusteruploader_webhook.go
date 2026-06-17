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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/validators"
)

// nolint:unused
// log is for logging in this package.
var clusteruploaderlog = logf.Log.WithName("clusteruploader-resource")

// SetupClusterUploaderWebhookWithManager registers the webhook for ClusterUploader in the manager.
func SetupClusterUploaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.ClusterUploader{}).
		WithValidator(&ClusterUploaderCustomValidator{
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
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clusteruploader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clusteruploaders,verbs=update;delete,versions=v1beta1,name=vclusteruploader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ClusterUploaderCustomValidator struct is responsible for validating the ClusterUploader resource
// when it is created, updated, or deleted.
type ClusterUploaderCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateCreate(_ context.Context, crawler *v1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("cluster uploader validate create should not be registered, see NOTE in webhook/v1beta1/clusteruploader_webhook.go", "name", crawler.GetName())
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateUpdate(ctx context.Context, oldUploader, newUploader *v1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("validating new parameters for cluster uploader", "name", newUploader.GetName())

	newRequiredParams := parseNewRequiredParameters(oldUploader.Spec.Parameters, newUploader.Spec.Parameters)

	dependantProfiles, err := v.getDependantProfiles(ctx, oldUploader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	for _, profile := range dependantProfiles {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if !refMatches(uploaderRef, oldUploader, "ClusterUploader") {
				continue
			}
			_, unset := validators.ParseSetParameters(uploaderRef, newRequiredParams)
			if len(unset) > 0 {
				var missingParamNames []string
				for _, u := range unset {
					missingParamNames = append(missingParamNames, u.Name)
				}
				return nil, apierrors.NewForbidden(
					schema.GroupResource{Group: v1beta1.Group, Resource: oldUploader.Name},
					oldUploader.Name, fmt.Errorf("dependant pipeline %s/%s does not define newly required parameters: [%s]", profile.Namespace, profile.Name, strings.Join(missingParamNames, ",")))
			}
		}

	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterUploader.
func (v *ClusterUploaderCustomValidator) ValidateDelete(ctx context.Context, uploader *v1beta1.ClusterUploader) (admission.Warnings, error) {
	clusteruploaderlog.Info("validation for cluster uploader upon deletion", "name", uploader.GetName())

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
			schema.GroupResource{Group: v1beta1.Group, Resource: uploader.Name}, uploader.Name,
			fmt.Errorf("cannot delete uploader with dependant profiles: [%s]", strings.Join(profileNames, ",")))
	}

	return nil, nil
}

func (v *ClusterUploaderCustomValidator) getDependantProfiles(ctx context.Context, uploader *v1beta1.ClusterUploader) ([]v1beta1.Profile, error) {
	var allProfiles v1beta1.ProfileList
	if err := v.c.List(ctx, &allProfiles); err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}

	var profiles []v1beta1.Profile
	for _, profile := range allProfiles.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			if refMatches(uploaderRef, uploader, "ClusterUploader") {
				profiles = append(profiles, profile)
				break
			}
		}
	}

	return profiles, nil
}
