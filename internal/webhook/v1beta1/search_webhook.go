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
var searchlog = logf.Log.WithName("search-resource")

// SetupSearchWebhookWithManager registers the webhook for Search in the manager.
func SetupSearchWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Search{}).
		WithValidator(&SearchCustomValidator{
			c: mgr.GetClient(),
		}).
		WithDefaulter(&SearchCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-search,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=searches,verbs=create;update,versions=v1beta1,name=msearch-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// SearchCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Search when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type SearchCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &SearchCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Search.
func (d *SearchCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	search, ok := obj.(*ocularcrashoverriderunv1beta1.Search)

	if !ok {
		return fmt.Errorf("expected an Search object but got %T", obj)
	}
	searchlog.Info("Defaulting for Search", "name", search.GetName())

	return nil
}

// NOTE: currently the search is only configured to run as a validating webhook
// during the update and/or creation of a Search resource to validate that
// 1) the referenced Crawler exists and is in the same namespace as the Search, and
// 2) all required parameters defined in the referenced Crawler are provided in the Search.
// Deletion validation is not currently needed because there are no resources that
// reference a Search. If in the future there is a need to validate Search resources
// on deletion, the ValidateDelete method below can be implemented and 'delete
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-search,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=searches,verbs=create;update,versions=v1beta1,name=vsearch-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// SearchCustomValidator struct is responsible for validating the Search resource
// when it is created, updated, or deleted.
type SearchCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &SearchCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Search.
func (v *SearchCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	search, ok := obj.(*ocularcrashoverriderunv1beta1.Search)
	if !ok {
		return nil, fmt.Errorf("expected a Search object but got %T", obj)
	}
	searchlog.Info("validating Search resource creation", "name", search.GetName())

	return nil, validateSearch(ctx, v.c, search)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Search.
func (v *SearchCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	search, ok := newObj.(*ocularcrashoverriderunv1beta1.Search)
	if !ok {
		return nil, fmt.Errorf("expected a Search object for the newObj but got %T", newObj)
	}
	searchlog.Info("validating Search resource update", "name", search.GetName())

	return nil, validateSearch(ctx, v.c, search)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Search.
func (v *SearchCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	search, ok := obj.(*ocularcrashoverriderunv1beta1.Search)
	if !ok {
		return nil, fmt.Errorf("expected a Search object but got %T", obj)
	}
	searchlog.Info("crawler validate delete should not be registered, see NOTE in webhook/v1beta1/search_webhook.go", "name", search.GetName())

	return nil, nil
}

func validateSearch(ctx context.Context, c client.Client, search *ocularcrashoverriderunv1beta1.Search) error {
	var allErrs field.ErrorList
	if search.Spec.CrawlerRef.Namespace != "" && search.Spec.CrawlerRef.Namespace != search.Namespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("crawlerRef").Child("namespace"), search.Spec.CrawlerRef.Namespace, "crawlerRef namespace must be empty or match the Search namespace"))
	}

	var crawler ocularcrashoverriderunv1beta1.Crawler
	if err := c.Get(ctx, client.ObjectKey{
		Name:      search.Spec.CrawlerRef.Name,
		Namespace: search.Namespace,
	}, &crawler); err != nil {
		return fmt.Errorf("failed to get referenced crawler %s: %w", search.Spec.CrawlerRef.Name, err)
	}
	paramErrs := validateSetParameters(crawler.Name, field.NewPath("spec").Child("crawlerRef").Child("parameters"), crawler.Spec.Parameters, search.Spec.Parameters)

	if len(paramErrs) > 0 {
		allErrs = append(allErrs, paramErrs...)
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Search"},
		search.Name, allErrs)
}
