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

	"github.com/hashicorp/go-multierror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-profile,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=profiles,verbs=create;update;delete,versions=v1beta1,name=vprofile-v1beta1.kb.io,admissionReviewVersions=v1

// ProfileCustomValidator struct is responsible for validating the Profile resource
// when it is created, updated, or deleted.
type ProfileCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &ProfileCustomValidator{}

func validateUploaderReferences(ctx context.Context, c client.Client, profile *ocularcrashoverriderunv1beta1.Profile) error {
	var (
		merr        *multierror.Error
		volumeNames = map[string]struct{}{}
	)
	for _, uploaderRef := range profile.Spec.UploaderRefs {
		if uploaderRef.Namespace != "" && uploaderRef.Namespace != profile.Namespace {
			merr = multierror.Append(merr, fmt.Errorf("profile %s references uploader %s in different namespace %s", profile.Name, uploaderRef.Name, uploaderRef.Namespace))
			continue
		}
		var uploader ocularcrashoverriderunv1beta1.Uploader
		if err := c.Get(ctx, client.ObjectKey{Name: uploaderRef.Name, Namespace: profile.Namespace}, &uploader); err != nil {
			if apierrors.IsNotFound(err) {
				merr = multierror.Append(merr, fmt.Errorf("uploader %s/%s not found", profile.Namespace, uploaderRef.Name))
				continue
			}
			merr = multierror.Append(merr, fmt.Errorf("failed to get uploader %s/%s: %w", profile.Namespace, uploaderRef.Name, err))
		}

		if err := validateSetParameters(uploaderRef.Name, uploader.Spec.Parameters, uploaderRef.Parameters); err != nil {
			merr = multierror.Append(merr, err)
		}

		for _, vol := range uploader.Spec.Volumes {
			if _, exists := volumeNames[vol.Name]; exists {
				merr = multierror.Append(merr, fmt.Errorf("volume name %s from uploader %s/%s is already used by another uploader in the profile %s/%s", vol.Name, profile.Namespace, uploaderRef.Name, profile.Namespace, profile.Name))
			} else {
				volumeNames[vol.Name] = struct{}{}
			}
		}
	}

	return merr.ErrorOrNil()
}

func validateSetParameters(name string, params []ocularcrashoverriderunv1beta1.ParameterDefinition, parameterSettings []ocularcrashoverriderunv1beta1.ParameterSetting) error {
	var merr *multierror.Error
	var setParams = map[string]struct{}{}
	for _, param := range parameterSettings {
		setParams[param.Name] = struct{}{}
	}
	for _, param := range params {
		if _, ok := setParams[param.Name]; !ok && param.Required {
			merr = multierror.Append(merr, fmt.Errorf("missing required parameter %s for instance %s", param.Name, name))
		}
	}
	return merr.ErrorOrNil()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object but got %T", obj)
	}

	if err := validateUploaderReferences(ctx, v.c, profile); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	profile, ok := newObj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object for the newObj but got %T", newObj)
	}
	profilelog.Info("Validation for Profile upon update", "name", profile.GetName())

	if err := validateUploaderReferences(ctx, v.c, profile); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Profile.
func (v *ProfileCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*ocularcrashoverriderunv1beta1.Profile)
	if !ok {
		return nil, fmt.Errorf("expected a Profile object but got %T", obj)
	}
	profilelog.Info("Validation for Profile upon deletion", "name", profile.GetName())

	var pipelines ocularcrashoverriderunv1beta1.PipelineList
	if err := v.c.List(ctx, &pipelines, client.InNamespace(profile.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	var merr *multierror.Error
	for _, pipeline := range pipelines.Items {
		if pipeline.Spec.ProfileRef.Name == profile.Name && pipeline.Namespace == profile.Namespace {
			merr = multierror.Append(merr, fmt.Errorf("profile %s/%s is still referenced by pipeline %s/%s", profile.Namespace, profile.Name, pipeline.Namespace, pipeline.Name))
		}
	}

	return nil, merr.ErrorOrNil()
}
