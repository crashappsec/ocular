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
var crawlerlog = logf.Log.WithName("crawler-resource")

// SetupCrawlerWebhookWithManager registers the webhook for Crawler in the manager.
func SetupCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Crawler{}).
		WithValidator(&CrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: currently the crawler is only configured to run as a validating webhook
// during the update and/or deletion of a Crawler resource to validate that
// 1) no new required parameters have been added that are not defined in
//    existing Search or CronSearch resources that reference it (on update), and
// 2) no Search or CronSearch resources referring to it exist (on delete).
// Creation is currently not needed since most of the work is handled by the
// k8s OpenAPI schema validation. If in the future there is a need to validate
// Crawler resources on creation, the ValidateCreate method below can be implemented and 'create'
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-crawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=crawlers,verbs=delete;update,versions=v1beta1,name=vcrawler-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// CrawlerCustomValidator struct is responsible for validating the Crawler resource
// when it is created, updated, or deleted.
type CrawlerCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &CrawlerCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	crawler, ok := obj.(*ocularcrashoverriderunv1beta1.Crawler)
	if !ok {
		return nil, fmt.Errorf("expected a Crawler object but got %T", obj)
	}
	crawlerlog.Info("crawler validate update should not be registered, see NOTE in webhook/v1beta1/crawler_webhook.go", "name", crawler.GetName())

	return nil, nil
}

func (v *CrawlerCustomValidator) validateNewRequiredParameters(ctx context.Context, oldCrawler, newCrawler *ocularcrashoverriderunv1beta1.Crawler) error {
	newRequiredParameters := validators.GetNewRequiredParameters(oldCrawler.Spec.Parameters, newCrawler.Spec.Parameters)

	var paramErrors field.ErrorList
	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}

	for _, search := range searches.Items {
		var namespace = search.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = search.Namespace
		}
		if search.Spec.CrawlerRef.Name == newCrawler.Name && namespace == newCrawler.Namespace {
			if !validators.AllParametersDefined(newRequiredParameters, search.Spec.Parameters) {
				paramErrors = append(paramErrors, field.Required(field.NewPath("spec").Child("parameters"), fmt.Sprintf("crawler %s is still referenced by search %s and not all new required parameters are defined", newCrawler.Name, search.Name)))
			}
		}
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches); err != nil {
		return fmt.Errorf("failed to list cron searches: %w", err)
	}

	for _, cSearch := range cronSearches.Items {
		var namespace = cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = cSearch.Namespace
		}
		if cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Name == newCrawler.Name && namespace == newCrawler.Namespace {
			if !validators.AllParametersDefined(newRequiredParameters, cSearch.Spec.SearchTemplate.Spec.Parameters) {
				paramErrors = append(paramErrors, field.Required(field.NewPath("spec").Child("parameters"), fmt.Sprintf("crawler %s is still referenced by cron search %s and not all new required parameters are defined", newCrawler.Name, cSearch.Name)))
			}
		}
	}

	if len(paramErrors) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Crawler"},
		newCrawler.Name, paramErrors)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	crawler, ok := newObj.(*ocularcrashoverriderunv1beta1.Crawler)
	if !ok {
		return nil, fmt.Errorf("expected a Crawler object for the newObj but got %T", newObj)
	}

	oldCrawler, ok := oldObj.(*ocularcrashoverriderunv1beta1.Crawler)
	if !ok {
		return nil, fmt.Errorf("expected a Uploader object for the oldObj but got %T", oldObj)
	}

	crawlerlog.Info("validating crawler update", "name", crawler.GetName())

	return nil, v.validateNewRequiredParameters(ctx, oldCrawler, crawler)
}

func (v *CrawlerCustomValidator) validateNoCrawlerReferences(ctx context.Context, crawler *ocularcrashoverriderunv1beta1.Crawler) error {
	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}
	var allErrs field.ErrorList
	for _, search := range searches.Items {
		var namespace = search.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = search.Namespace
		}
		if search.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("crawlerRef"), fmt.Sprintf("crawler %s is still referenced by search %s", crawler.Name, search.Name)))
		}
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches); err != nil {
		return fmt.Errorf("failed to list cron searches: %w", err)
	}

	for _, cSearch := range cronSearches.Items {
		var namespace = cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = cSearch.Namespace
		}
		if cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("crawlerRef"), fmt.Sprintf("crawler %s is still referenced by cron search %s", crawler.Name, cSearch.Name)))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Crawler"},
		crawler.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	crawler, ok := obj.(*ocularcrashoverriderunv1beta1.Crawler)
	if !ok {
		return nil, fmt.Errorf("expected a Crawler object but got %T", obj)
	}
	crawlerlog.Info("validating crawler is no longer referenced by any Search or CronSearch resource", "name", crawler.GetName())

	return nil, v.validateNoCrawlerReferences(ctx, crawler)
}
