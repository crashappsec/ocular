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

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-search,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=searches,verbs=create;update,versions=v1beta1,name=msearch-v1beta1.kb.io,admissionReviewVersions=v1

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

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-search,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=searches,verbs=create;update,versions=v1beta1,name=vsearch-v1beta1.kb.io,admissionReviewVersions=v1

// SearchCustomValidator struct is responsible for validating the Search resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
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
	searchlog.Info("Validation for Search upon creation", "name", search.GetName())

	return nil, validateSearch(ctx, v.c, search)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Search.
func (v *SearchCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	search, ok := newObj.(*ocularcrashoverriderunv1beta1.Search)
	if !ok {
		return nil, fmt.Errorf("expected a Search object for the newObj but got %T", newObj)
	}
	searchlog.Info("Validation for Search upon update", "name", search.GetName())

	return nil, validateSearch(ctx, v.c, search)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Search.
func (v *SearchCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	search, ok := obj.(*ocularcrashoverriderunv1beta1.Search)
	if !ok {
		return nil, fmt.Errorf("expected a Search object but got %T", obj)
	}
	searchlog.Info("Validation for Search upon deletion", "name", search.GetName())

	return nil, nil
}

func validateSearch(ctx context.Context, c client.Client, search *ocularcrashoverriderunv1beta1.Search) error {
	if search.Spec.CrawlerRef.Namespace != "" || search.Spec.CrawlerRef.Namespace != search.Namespace {
		return fmt.Errorf("crawlerRef.namespace must be empty or match the namespace of the Search")
	}

	var crawler ocularcrashoverriderunv1beta1.Crawler
	if err := c.Get(ctx, client.ObjectKey{
		Name:      search.Spec.CrawlerRef.Name,
		Namespace: search.Namespace,
	}, &crawler); err != nil {
		return fmt.Errorf("failed to get referenced crawler %s: %w", search.Spec.CrawlerRef.Name, err)
	}

	if err := validateSetParameters(crawler.Name, crawler.Spec.Parameters, search.Spec.Parameters); err != nil {
		return err
	}

	return nil
}
