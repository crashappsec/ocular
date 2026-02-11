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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	"github.com/hashicorp/go-multierror"
)

// nolint:unused
// log is for logging in this package.
var clustercrawlerlog = logf.Log.WithName("clustercrawler-resource")

// SetupClusterCrawlerWebhookWithManager registers the webhook for ClusterCrawler in the manager.
func SetupClusterCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.ClusterCrawler{}).
		WithValidator(&ClusterCrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: this validator is currently only enabled for 'delete'.
// additional options can be specified in the 'verbs' parameter
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clustercrawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clustercrawlers,verbs=delete,versions=v1beta1,name=vclustercrawler-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

type ClusterCrawlerCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateCreate(_ context.Context, obj *ocularcrashoverriderunv1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("cluster crawler validate create should not be registered, see NOTE in webhook/v1beta1/clustercrawler_webhook.go", "name", obj.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *ocularcrashoverriderunv1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("cluster crawler validate update should not be registered, see NOTE in webhook/v1beta1/clustercrawler_webhook.go", "name", newObj.GetName())

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateDelete(ctx context.Context, obj *ocularcrashoverriderunv1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("validation for cluster crawler upon deletion", "name", obj.GetName())

	return nil, v.validateNoClusterCrawlerReferences(ctx, obj)
}

func (v *ClusterCrawlerCustomValidator) validateNoClusterCrawlerReferences(ctx context.Context, crawler *ocularcrashoverriderunv1beta1.ClusterCrawler) error {
	var searches ocularcrashoverriderunv1beta1.SearchList
	if err := v.c.List(ctx, &searches); err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}

	var cronSearches ocularcrashoverriderunv1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearches); err != nil {
		return fmt.Errorf("failed to list cron searches: %w", err)
	}
	var allErrs error
	for _, search := range searches.Items {
		crawlerRef := search.Spec.CrawlerRef
		if crawlerRef.Name == crawler.Name && crawlerRef.Kind == "ClusterCrawler" {
			allErrs = multierror.Append(allErrs, fmt.Errorf("this resource cannot be deleted because it is still referenced by 'Search/%s' in namespace '%s'",
				search.Name, search.Namespace))
		}
	}

	for _, cSearch := range cronSearches.Items {
		crawlerRef := cSearch.Spec.SearchTemplate.Spec.CrawlerRef
		if crawlerRef.Name == crawler.Name && crawlerRef.Kind == "ClusterCrawler" {
			allErrs = multierror.Append(allErrs, fmt.Errorf("this resource cannot be deleted because it is still referenced by 'CronSearch/%s' in namespace '%s'",
				cSearch.Name, cSearch.Namespace))
		}
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewForbidden(
		schema.GroupResource{Group: "ocular.crashoverride.run", Resource: crawler.Name},
		crawler.Name, allErrs)
}
