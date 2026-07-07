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

	"github.com/crashappsec/ocular/internal/validators"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var crawlerlog = logf.Log.WithName("crawler-resource")

// SetupCrawlerWebhookWithManager registers the webhook for Crawler in the manager.
func SetupCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.Crawler{}).
		WithValidator(&CrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: currently the crawler only configured to run as a validating webhook
// during the update and/or deletion of a Crawler resource to validate that
// 1) no new required parameters have been added that are not defined in
//    existing (Cron)?Search resources that reference it (on update), and
// 2) no (Cron)?Search resources referring to it exist (on delete).
// 3) validate entrypoint is set on container (update/create)
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-crawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=crawlers,verbs=create;delete;update,versions=v1beta1,name=vcrawler-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// CrawlerCustomValidator struct is responsible for validating the Crawler resource
// when it is created, updated, or deleted.
type CrawlerCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateCreate(ctx context.Context, crawler *v1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validating crawler upon creation", "name", crawler.GetName())

	fieldErrs := validators.ValidateContainerDefinition(ctx, field.NewPath("spec").Child("container"), crawler.Spec.Container)

	if len(fieldErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: v1beta1.Group, Kind: "Crawler"},
		crawler.Name, fieldErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateUpdate(ctx context.Context, oldCrawler, newCrawler *v1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validation for crawler upon update", "name", newCrawler.GetName())

	newRequiredParams := parseNewRequiredParameters(oldCrawler.Spec.Parameters, newCrawler.Spec.Parameters)

	dependantSearches, err := v.getDependantSearches(ctx, oldCrawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	for _, search := range dependantSearches {
		_, unset := validators.ParseSetParameters(search.Spec.CrawlerRef, newRequiredParams)
		if len(unset) > 0 {
			var missingParamNames []string
			for _, u := range unset {
				missingParamNames = append(missingParamNames, u.Name)
			}
			return nil, apierrors.NewForbidden(
				schema.GroupResource{Group: v1beta1.Group, Resource: newCrawler.Name},
				newCrawler.Name, fmt.Errorf("dependant search %s does not define newly required parameters: [%s]", search.Name, strings.Join(missingParamNames, ",")))
		}
	}

	dependantCronSearches, err := v.getDependantCronSearches(ctx, oldCrawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	for _, cronsearch := range dependantCronSearches {
		_, unset := validators.ParseSetParameters(cronsearch.Spec.SearchTemplate.Spec.CrawlerRef, newRequiredParams)
		if len(unset) > 0 {
			var missingParamNames []string
			for _, u := range unset {
				missingParamNames = append(missingParamNames, u.Name)
			}
			return nil, apierrors.NewForbidden(
				schema.GroupResource{Group: v1beta1.Group, Resource: newCrawler.Name},
				newCrawler.Name, fmt.Errorf("dependant cronsearch %s does not define newly required parameters: [%s]", cronsearch.Name, strings.Join(missingParamNames, ",")))
		}
	}

	fieldErrs := validators.ValidateContainerDefinition(ctx, field.NewPath("spec").Child("container"), newCrawler.Spec.Container)

	if len(fieldErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: v1beta1.Group, Kind: "Crawler"},
		newCrawler.Name, fieldErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Crawler.
func (v *CrawlerCustomValidator) ValidateDelete(ctx context.Context, crawler *v1beta1.Crawler) (admission.Warnings, error) {
	crawlerlog.Info("validating crawler is no longer referenced by any Search or CronSearch resource", "name", crawler.GetName())

	dependantSearches, err := v.getDependantSearches(ctx, crawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantSearches) > 0 {
		searchNames := make([]string, 0, len(dependantSearches))
		for _, search := range dependantSearches {
			searchNames = append(searchNames, search.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: v1beta1.Group, Resource: crawler.Name}, crawler.Name,
			fmt.Errorf("cannot delete crawler with dependant searches: [%s]", strings.Join(searchNames, ",")))
	}

	dependantCronSearches, err := v.getDependantCronSearches(ctx, crawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantCronSearches) > 0 {
		cronSearchNames := make([]string, 0, len(dependantCronSearches))
		for _, cronSearch := range dependantCronSearches {
			cronSearchNames = append(cronSearchNames, cronSearch.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: v1beta1.Group, Resource: crawler.Name}, crawler.Name,
			fmt.Errorf("cannot delete crawler with dependant cron searches: [%s]", strings.Join(cronSearchNames, ",")))
	}

	return nil, nil
}

func (v *CrawlerCustomValidator) getDependantSearches(ctx context.Context, crawler *v1beta1.Crawler) ([]v1beta1.Search, error) {
	var searchesInNamespace v1beta1.SearchList
	if err := v.c.List(ctx, &searchesInNamespace, client.InNamespace(crawler.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list searches in namespace %s: %w", crawler.Namespace, err)
	}

	var searches []v1beta1.Search
	for _, search := range searchesInNamespace.Items {
		if refMatches(search.Spec.CrawlerRef, crawler, "Crawler") {
			searches = append(searches, search)
			break
		}
	}
	return searches, nil
}

func (v *CrawlerCustomValidator) getDependantCronSearches(ctx context.Context, crawler *v1beta1.Crawler) ([]v1beta1.CronSearch, error) {
	var cronSearchesInNamespace v1beta1.CronSearchList
	if err := v.c.List(ctx, &cronSearchesInNamespace, client.InNamespace(crawler.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list cronSearches in namespace %s: %w", crawler.Namespace, err)
	}

	var cronSearches []v1beta1.CronSearch
	for _, cronSearch := range cronSearchesInNamespace.Items {
		if refMatches(cronSearch.Spec.SearchTemplate.Spec.CrawlerRef, crawler, "Crawler") {
			cronSearches = append(cronSearches, cronSearch)
			break
		}
	}
	return cronSearches, nil
}
