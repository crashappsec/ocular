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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/validators"
)

// nolint:unused
// log is for logging in this package.
var downloaderlog = logf.Log.WithName("downloader-resource")

// SetupDownloaderWebhookWithManager registers the webhook for Downloader in the manager.
func SetupDownloaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.Downloader{}).
		WithValidator(&DownloaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: currently the downloader only configured to run as a validating webhook
// during the update and/or deletion of a Downloader resource to validate that
// 1) no new required parameters have been added that are not defined in
//    existing Pipeline resources that reference it (on update), and
// 2) no Pipeline resources referring to it exist (on delete).
// Creation is currently not needed since most of the work is handled by the
// k8s OpenAPI schema validation. If in the future there is a need to validate
// Pipeline resources on creation, the ValidateCreate method below can be implemented and 'create'
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-downloader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=downloaders,verbs=update;delete,versions=v1beta1,name=vdownloader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// DownloaderCustomValidator struct is responsible for validating the Downloader resource
// when it is created, updated, or deleted.
type DownloaderCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateCreate(_ context.Context, downloader *v1beta1.Downloader) (admission.Warnings, error) {
	downloaderlog.Info("downloader validate create should not be registered, see NOTE in webhook/v1beta1/downloader_webhook.go", "name", downloader.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateUpdate(ctx context.Context, oldDownloader, newDownloader *v1beta1.Downloader) (admission.Warnings, error) {
	downloaderlog.Info("validation for downloader upon update", "name", newDownloader.GetName())

	newRequiredParams := parseNewRequiredParameters(oldDownloader.Spec.Parameters, newDownloader.Spec.Parameters)

	dependantPipelines, err := v.getDependantPipelines(ctx, oldDownloader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	for _, pipeline := range dependantPipelines {
		_, unset := validators.ParseSetParameters(pipeline.Spec.DownloaderRef, newRequiredParams)
		if len(unset) > 0 {
			var missingParamNames []string
			for _, u := range unset {
				missingParamNames = append(missingParamNames, u.Name)
			}
			return nil, apierrors.NewForbidden(
				schema.GroupResource{Group: v1beta1.Group, Resource: newDownloader.Name},
				newDownloader.Name, fmt.Errorf("dependant pipeline %s does not define newly required parameters: [%s]", pipeline.Name, strings.Join(missingParamNames, ",")))
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateDelete(ctx context.Context, downloader *v1beta1.Downloader) (admission.Warnings, error) {
	downloaderlog.Info("validation for downloader upon deletion", "name", downloader.GetName())
	dependantPipelines, err := v.getDependantPipelines(ctx, downloader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantPipelines) > 0 {
		pipelineNames := make([]string, 0, len(dependantPipelines))
		for _, pipeline := range dependantPipelines {
			pipelineNames = append(pipelineNames, pipeline.Name)
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: v1beta1.Group, Resource: downloader.Name}, downloader.Name,
			fmt.Errorf("cannot delete downloader with dependant pipelines: [%s]", strings.Join(pipelineNames, ",")))
	}

	return nil, nil
}

func (v *DownloaderCustomValidator) getDependantPipelines(ctx context.Context, downloader *v1beta1.Downloader) ([]v1beta1.Pipeline, error) {
	var pipelinesInNamespace v1beta1.PipelineList
	if err := v.c.List(ctx, &pipelinesInNamespace, client.InNamespace(downloader.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pipelines in namespace %s: %w", downloader.Namespace, err)
	}

	var pipelines []v1beta1.Pipeline
	for _, pipeline := range pipelinesInNamespace.Items {
		if refMatches(pipeline.Spec.DownloaderRef, downloader, "Downloader") {
			pipelines = append(pipelines, pipeline)
			break
		}
	}
	return pipelines, nil
}
