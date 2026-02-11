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
	"slices"

	"github.com/crashappsec/ocular/internal/validators"
	"github.com/hashicorp/go-multierror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var crawlerlog = logf.Log.WithName("crawler-resource")

// SetupCrawlerWebhookWithManager registers the webhook for Crawler in the manager.
func SetupCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.Crawler{}).
		WithValidator(&CrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-crawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=crawlers,verbs=create;delete;update,versions=v1beta1,name=vcrawler-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// CrawlerCustomValidator struct is responsible for validating the Crawler resource
// when it is created, updated, or deleted.
type CrawlerCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateCreate(ctx context.Context, crawler *ocularcrashoverriderunv1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validating crawler", "name", crawler.GetName())

	return nil, v.validateCrawler(ctx, crawler)
}

func (v *CrawlerCustomValidator) validateCrawler(_ context.Context, crawler *ocularcrashoverriderunv1beta1.Crawler) error {

	var allErrs field.ErrorList
	allErrs = append(allErrs, validators.ValidateAdditionalLabels(crawler.Spec.AdditionalPodMetadata.Labels, field.NewPath("spec").Child("additionalPodMetadata").Child("labels"))...)

	allErrs = append(allErrs, validators.ValidateAdditionalAnnotations(crawler.Spec.AdditionalPodMetadata.Annotations, field.NewPath("spec").Child("additionalPodMetadata").Child("annotations"))...)
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Crawler"}, crawler.Name, allErrs)

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
			if !validators.AllParametersDefined(newRequiredParameters, search.Spec.CrawlerRef.Parameters) {
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
			if !validators.AllParametersDefined(newRequiredParameters, cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Parameters) {
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
func (v *CrawlerCustomValidator) ValidateUpdate(ctx context.Context, oldCrawler, newCrawler *ocularcrashoverriderunv1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validation for crawler upon update", "name", newCrawler.GetName())

	crawlerlog.Info("validating crawler update", "name", newCrawler.GetName())
	if err := v.validateCrawler(ctx, newCrawler); err != nil {
		return nil, err
	}

	return nil, v.validateNewRequiredParameters(ctx, oldCrawler, newCrawler)
}

func (v *CrawlerCustomValidator) validateNoCrawlerReferences(ctx context.Context, crawler *ocularcrashoverriderunv1beta1.Crawler) error {
	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches, client.InNamespace(crawler.Namespace)); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches, client.InNamespace(crawler.Namespace)); err != nil {
		return fmt.Errorf("failed to list cron searches: %w", err)
	}
	var allErrs error
	for _, search := range searches.Items {
		// ignore cluster crawler
		if !slices.Contains([]string{"", "Crawler"}, search.Spec.CrawlerRef.Kind) {
			continue
		}
		crawlerRef := search.Spec.CrawlerRef
		if crawlerRef.Name == crawler.Name && crawlerRef.Namespace == crawler.Namespace {
			allErrs = multierror.Append(allErrs, fmt.Errorf("this resource cannot be deleted because it is still referenced by 'Search/%s in namespace %s'", search.Name, search.Namespace))
		}
	}

	for _, cSearch := range cronSearches.Items {
		// ignore cluster crawlers
		if !slices.Contains([]string{"", "Crawler"}, cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Kind) {
			continue
		}
		crawlerRef := cSearch.Spec.SearchTemplate.Spec.CrawlerRef
		if crawlerRef.Name == crawler.Name && crawlerRef.Namespace == crawler.Namespace {
			allErrs = multierror.Append(allErrs, fmt.Errorf("this resource cannot be deleted because it is still referenced by 'CronSearch/%s in namespace %s'", cSearch.Name, cSearch.Namespace))
		}
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewForbidden(
		schema.GroupResource{Group: "ocular.crashoverride.run", Resource: crawler.Name},
		crawler.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateDelete(ctx context.Context, crawler *ocularcrashoverriderunv1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validating crawler is no longer referenced by any Search or CronSearch resource", "name", crawler.GetName())

	return nil, v.validateNoCrawlerReferences(ctx, crawler)
}
