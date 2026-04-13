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
var clusterdownloaderlog = logf.Log.WithName("clusterdownloader-resource")

// SetupClusterDownloaderWebhookWithManager registers the webhook for ClusterDownloader in the manager.
func SetupClusterDownloaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.ClusterDownloader{}).
		WithValidator(&ClusterDownloaderCustomValidator{
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
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clusterdownloader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clusterdownloaders,verbs=update;delete,versions=v1beta1,name=vclusterdownloader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ClusterDownloaderCustomValidator struct is responsible for validating the ClusterDownloader resource
type ClusterDownloaderCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateCreate(_ context.Context, obj *v1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("cluster downloader validate create should not be registered, see NOTE in webhook/v1beta1/clusterdownloader_webhook.go", "name", obj.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateUpdate(ctx context.Context, oldDownloader, newDownloader *v1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("validation for downloader upon update", "name", newDownloader.GetName())

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
				schema.GroupResource{Group: "ocular.crashoverride.run", Resource: newDownloader.Name},
				newDownloader.Name, fmt.Errorf("dependant pipeline %s/%s does not define newly required parameters: [%s]", pipeline.Namespace, pipeline.Name, strings.Join(missingParamNames, ",")))
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateDelete(ctx context.Context, downloader *v1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("validation for cluster downloader upon deletion", "name", downloader.GetName())
	dependantPipelines, err := v.getDependantPipelines(ctx, downloader)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if len(dependantPipelines) > 0 {
		pipelineNames := make([]string, 0, len(dependantPipelines))
		for _, pipeline := range dependantPipelines {
			pipelineNames = append(pipelineNames, fmt.Sprintf("%s/%s", pipeline.Namespace, pipeline.Name))
		}
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "ocular.crashoverride.run", Resource: downloader.Name}, downloader.Name,
			fmt.Errorf("cannot delete cluster downloader with dependant pipelines: [%s]", strings.Join(pipelineNames, ",")))
	}

	return nil, nil
}

func (v *ClusterDownloaderCustomValidator) getDependantPipelines(ctx context.Context, downloader *v1beta1.ClusterDownloader) ([]v1beta1.Pipeline, error) {
	var allPipelines v1beta1.PipelineList
	if err := v.c.List(ctx, &allPipelines, client.InNamespace(downloader.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pipelines in namespace %s: %w", downloader.Namespace, err)
	}

	var pipelines []v1beta1.Pipeline
	for _, pipeline := range allPipelines.Items {
		if refMatches(pipeline.Spec.DownloaderRef, downloader, "ClusterDownloader") {
			pipelines = append(pipelines, pipeline)
			break
		}
	}
	return pipelines, nil
}
