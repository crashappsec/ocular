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
var crawlerlog = logf.Log.WithName("crawler-resource")

// SetupCrawlerWebhookWithManager registers the webhook for Crawler in the manager.
func SetupCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Crawler{}).
		WithValidator(&CrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-crawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=crawlers,verbs=delete;update,versions=v1beta1,name=vcrawler-v1beta1.kb.io,admissionReviewVersions=v1

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
	crawlerlog.Info("Validation for Crawler upon creation", "name", crawler.GetName())

	return nil, nil
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

	newRequiredParameters := getNewRequiredParameters(oldCrawler.Spec.Parameters, crawler.Spec.Parameters)

	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches); err != nil {
		return nil, fmt.Errorf("failed to list searches: %w", err)
	}
	var merr *multierror.Error
	for _, search := range searches.Items {
		var namespace = search.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = search.Namespace
		}
		if search.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			if !allParametersDefined(newRequiredParameters, search.Spec.Parameters) {
				err := fmt.Errorf("crawler %s is still referenced by search %s and not all new required parameters are defined", crawler.Name, search.Name)
				merr = multierror.Append(merr, err)
			}
		}
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches); err != nil {
		return nil, fmt.Errorf("failed to list cron searches: %w", err)
	}

	for _, cSearch := range cronSearches.Items {
		var namespace = cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = cSearch.Namespace
		}
		if cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			if !allParametersDefined(newRequiredParameters, cSearch.Spec.SearchTemplate.Spec.Parameters) {
				err := fmt.Errorf("crawler %s is still referenced by cron search %s and not all new required parameters are defined", crawler.Name, cSearch.Name)
				merr = multierror.Append(merr, err)
			}
		}
	}

	return nil, merr.ErrorOrNil()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	crawler, ok := obj.(*ocularcrashoverriderunv1beta1.Crawler)
	if !ok {
		return nil, fmt.Errorf("expected a Crawler object but got %T", obj)
	}
	crawlerlog.Info("Validation for Crawler upon deletion", "name", crawler.GetName())

	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches); err != nil {
		return nil, fmt.Errorf("failed to list searches: %w", err)
	}
	var merr *multierror.Error
	for _, search := range searches.Items {
		var namespace = search.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = search.Namespace
		}
		if search.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			merr = multierror.Append(merr, fmt.Errorf("crawler %s is still referenced by search %s", crawler.Name, search.Name))
		}
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches); err != nil {
		return nil, fmt.Errorf("failed to list cron searches: %w", err)
	}

	for _, cSearch := range cronSearches.Items {
		var namespace = cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Namespace
		if namespace == "" {
			namespace = cSearch.Namespace
		}
		if cSearch.Spec.SearchTemplate.Spec.CrawlerRef.Name == crawler.Name && namespace == crawler.Namespace {
			merr = multierror.Append(merr, fmt.Errorf("crawler %s is still referenced by cron search %s", crawler.Name, cSearch.Name))
		}
	}

	return nil, merr.ErrorOrNil()
}
