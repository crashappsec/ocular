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
var uploaderlog = logf.Log.WithName("uploader-resource")

// SetupUploaderWebhookWithManager registers the webhook for Uploader in the manager.
func SetupUploaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Uploader{}).
		WithValidator(&UploaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-uploader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=uploaders,verbs=delete;update,versions=v1beta1,name=vuploader-v1beta1.kb.io,admissionReviewVersions=v1

// UploaderCustomValidator struct is responsible for validating the Uploader resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
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

	uploaderlog.Info("Validation for Uploader upon creation", "name", uploader.GetName())

	return nil, nil
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

	newRequiredParameters := getNewRequiredParameters(oldUploader.Spec.Parameters, uploader.Spec.Parameters)

	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	var merr *multierror.Error
	for _, profile := range profiles.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			var namespace = uploaderRef.Namespace
			if namespace == "" {
				namespace = profile.Namespace
			}
			if uploaderRef.Name == uploader.Name && namespace == uploader.Namespace {
				if !allParametersDefined(newRequiredParameters, uploaderRef.Parameters) {
					err := fmt.Errorf("uploader %s is still referenced by profile %s and not all new required parameters are defined", uploader.Name, profile.Name)
					merr = multierror.Append(merr, err)
				}
			}
		}
	}

	return nil, nil
}

func allParametersDefined(paramNames []string, paramValues []ocularcrashoverriderunv1beta1.ParameterSetting) bool {
	var definedParams = make(map[string]struct{}, len(paramNames))
	for _, paramValue := range paramValues {
		definedParams[paramValue.Name] = struct{}{}
	}
	for _, paramName := range paramNames {
		if _, exists := definedParams[paramName]; !exists {
			return false
		}
	}
	return true
}

func getNewRequiredParameters(oldParams, newParams []ocularcrashoverriderunv1beta1.ParameterDefinition) []string {
	result := make([]string, 0, len(oldParams)+len(newParams))
	var newRequiredParameters = make(map[string]ocularcrashoverriderunv1beta1.ParameterDefinition, len(newParams))
	for _, paramDef := range newParams {
		if paramDef.Required {
			newRequiredParameters[paramDef.Name] = paramDef
		}
	}

	for _, oldParamDef := range oldParams {
		if oldParamDef.Required {
			delete(newRequiredParameters, oldParamDef.Name)
		}
	}

	for paramName := range newRequiredParameters {
		result = append(result, paramName)
	}
	return result
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Uploader.
func (v *UploaderCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	uploader, ok := obj.(*ocularcrashoverriderunv1beta1.Uploader)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object but got %T", obj)
	}

	var profiles ocularcrashoverriderunv1beta1.ProfileList
	if err := v.c.List(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	var merr *multierror.Error
	for _, profile := range profiles.Items {
		for _, uploaderRef := range profile.Spec.UploaderRefs {
			var namespace = uploaderRef.Namespace
			if namespace == "" {
				namespace = profile.Namespace
			}
			if uploaderRef.Name == uploader.Name && namespace == uploader.Namespace {
				profilelog.Info("found profile reference to uploader", "profile", profile.GetName(), "name", uploader.GetName())
				merr = multierror.Append(merr, fmt.Errorf("uploader %s is still referenced by profile %s", uploader.Name, profile.Name))
			}
		}
	}

	return nil, merr.ErrorOrNil()
}
