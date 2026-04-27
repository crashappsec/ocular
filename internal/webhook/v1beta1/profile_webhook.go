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

	"github.com/crashappsec/ocular/internal/resources"
	"github.com/crashappsec/ocular/internal/validators"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var profilelog = logf.Log.WithName("profile-resource")

// SetupProfileWebhookWithManager registers the webhook for Profile in the manager.
func SetupProfileWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.Profile{}).
		WithValidator(&ProfileCustomValidator{
			c: mgr.GetClient(),
		}).
		WithDefaulter(&ProfileCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-profile,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=profile,verbs=create;update,versions=v1beta1,name=mprofile-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pipeline when those are created or updated.
type ProfileCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Pipeline.
func (d *ProfileCustomDefaulter) Default(_ context.Context, profile *v1beta1.Profile) error {
	profilelog.Info("defaulting for profile", "name", profile.GetName())

	for i, uploaderRef := range profile.Spec.UploaderRefs {
		uploaderRef = resources.ReferenceDefaulter(uploaderRef, "Uploader")
		profile.Spec.UploaderRefs[i] = uploaderRef
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-profile,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=profiles,verbs=create;update;delete,versions=v1beta1,name=vprofile-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ProfileCustomValidator struct is responsible for validating the Profile resource
// when it is created, updated, or deleted.
type ProfileCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateCreate(ctx context.Context, profile *v1beta1.Profile) (admission.Warnings, error) {

	return nil, validators.ValidateProfile(ctx, v.c, profile)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateUpdate(ctx context.Context, oldProfile, newProfile *v1beta1.Profile) (admission.Warnings, error) {
	if err := validators.ValidateProfile(ctx, v.c, newProfile); err != nil {
		return nil, err
	}

	newRequiredParams := parseNewRequiredParameters(oldProfile.Spec.Parameters, newProfile.Spec.Parameters)

	dependantPipelines, err := v.getPipelineReferences(ctx, oldProfile)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	for _, pipeline := range dependantPipelines {
		_, unset := validators.ParseSetParameters(pipeline.Spec.ProfileRef, newRequiredParams)
		if len(unset) > 0 {
			var missingParamNames []string
			for _, u := range unset {
				missingParamNames = append(missingParamNames, u.Name)
			}
			return nil, apierrors.NewForbidden(
				schema.GroupResource{Group: "ocular.crashoverride.run", Resource: newProfile.Name},
				newProfile.Name, fmt.Errorf("dependant pipeline %s does not define newly required parameters: [%s]", pipeline.Name, strings.Join(missingParamNames, ",")))
		}

	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateDelete(ctx context.Context, profile *v1beta1.Profile) (admission.Warnings, error) {
	profilelog.Info("Validation for Profile upon deletion", "name", profile.GetName())

	pipelines, err := v.getPipelineReferences(ctx, profile)
	if err != nil {
		return nil, err
	}

	if len(pipelines) != 0 {
		pipelineNames := make([]string, 0, len(pipelines))
		for _, pipeline := range pipelines {
			pipelineNames = append(pipelineNames, pipeline.GetName())
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "ocular.crashoverride.run", Resource: profile.Name},
			profile.Name,
			fmt.Errorf("cannot delete profile still used by active pipelines in namespace %s: [%s]", profile.Namespace, strings.Join(pipelineNames, ", ")),
		)
	}

	return nil, nil
}

func (v *ProfileCustomValidator) getPipelineReferences(ctx context.Context, profile *v1beta1.Profile) ([]v1beta1.Pipeline, error) {
	var pipelinesInNamespace v1beta1.PipelineList
	if err := v.c.List(ctx, &pipelinesInNamespace, client.InNamespace(profile.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pipelines in namespace %s: %w", profile.Namespace, err)
	}

	var pipelines []v1beta1.Pipeline
	for _, pipeline := range pipelinesInNamespace.Items {
		if pipeline.Spec.ProfileRef.Name == profile.Name && pipeline.Namespace == profile.Namespace {
			pipelines = append(pipelines, pipeline)
		}
	}

	return pipelines, nil
}
