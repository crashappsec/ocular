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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/validators"
)

// nolint:unused
// log is for logging in this package.
var clustercrawlerlog = logf.Log.WithName("clustercrawler-resource")

// SetupClusterCrawlerWebhookWithManager registers the webhook for ClusterCrawler in the manager.
func SetupClusterCrawlerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.ClusterCrawler{}).
		WithValidator(&ClusterCrawlerCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: the validator is configured to run as a validating webhook
// during the create, update and/or deletion of a ClusterCrawler resource to validate that
// 1) no new required parameters have been added that are not defined in
//    existing (Cron)?Search resources that reference it (on update), and
// 2) no (Cron)?Search resources referring to it exist (on delete).
// 3) the crawler specified an entrypoint (on update/create)
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clustercrawler,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clustercrawlers,verbs=create;update;delete,versions=v1beta1,name=vclustercrawler-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

type ClusterCrawlerCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateCreate(ctx context.Context, obj *v1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("valdiating cluster crawler creation", "name", obj.GetName())

	fieldErrs := validators.ValidateContainerDefinition(ctx, field.NewPath("spec").Child("container"), obj.Spec.Container)

	if len(fieldErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: v1beta1.Group, Kind: "ClusterCrawler"},
		obj.Name, fieldErrs)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateUpdate(ctx context.Context, oldCrawler, newCrawler *v1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("validation for cluster crawler upon update", "name", newCrawler.GetName())

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
		schema.GroupKind{Group: v1beta1.Group, Kind: "ClusterCrawler"},
		newCrawler.Name, fieldErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterCrawler.
func (v *ClusterCrawlerCustomValidator) ValidateDelete(ctx context.Context, clusterCrawler *v1beta1.ClusterCrawler) (admission.Warnings, error) {
	clustercrawlerlog.Info("validation for cluster crawler upon deletion", "name", clusterCrawler.GetName())

	dependantSearches, err := v.getDependantSearches(ctx, clusterCrawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantSearches) > 0 {
		searchNames := make([]string, 0, len(dependantSearches))
		for _, search := range dependantSearches {
			searchNames = append(searchNames, search.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: v1beta1.Group, Resource: clusterCrawler.Name}, clusterCrawler.Name,
			fmt.Errorf("cannot delete cluster crawler with dependant searches: [%s]", strings.Join(searchNames, ",")))
	}

	dependantCronSearches, err := v.getDependantCronSearches(ctx, clusterCrawler)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantCronSearches) > 0 {
		cronSearchNames := make([]string, 0, len(dependantCronSearches))
		for _, cronSearch := range dependantCronSearches {
			cronSearchNames = append(cronSearchNames, cronSearch.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: v1beta1.Group, Resource: clusterCrawler.Name}, clusterCrawler.Name,
			fmt.Errorf("cannot delete crawler with dependant cron searches: [%s]", strings.Join(cronSearchNames, ",")))
	}

	return nil, nil
}

func (v *ClusterCrawlerCustomValidator) getDependantSearches(ctx context.Context, crawler *v1beta1.ClusterCrawler) ([]v1beta1.Search, error) {
	var all v1beta1.SearchList
	if err := v.c.List(ctx, &all); err != nil {
		return nil, fmt.Errorf("failed to list searches: %w", err)
	}

	var searches []v1beta1.Search
	for _, search := range all.Items {
		if refMatches(search.Spec.CrawlerRef, crawler, "ClusterCrawler") {
			searches = append(searches, search)
			break
		}
	}
	return searches, nil
}

func (v *ClusterCrawlerCustomValidator) getDependantCronSearches(ctx context.Context, crawler *v1beta1.ClusterCrawler) ([]v1beta1.CronSearch, error) {
	var all v1beta1.CronSearchList
	if err := v.c.List(ctx, &all); err != nil {
		return nil, fmt.Errorf("failed to list cronSearches: %w", err)
	}

	var dependant []v1beta1.CronSearch
	for _, cronSearch := range all.Items {
		if refMatches(cronSearch.Spec.SearchTemplate.Spec.CrawlerRef, crawler, "ClusterCrawler") {
			dependant = append(dependant, cronSearch)
			break
		}
	}
	return dependant, nil
}
