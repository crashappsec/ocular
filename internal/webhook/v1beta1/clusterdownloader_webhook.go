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
var clusterdownloaderlog = logf.Log.WithName("clusterdownloader-resource")

// SetupClusterDownloaderWebhookWithManager registers the webhook for ClusterDownloader in the manager.
func SetupClusterDownloaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.ClusterDownloader{}).
		WithValidator(&ClusterDownloaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: this validator is currently only enabled for 'delete'.
// additional options can be specified in the 'verbs' parameter
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-clusterdownloader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=clusterdownloaders,verbs=delete,versions=v1beta1,name=vclusterdownloader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// ClusterDownloaderCustomValidator struct is responsible for validating the ClusterDownloader resource
type ClusterDownloaderCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateCreate(_ context.Context, obj *ocularcrashoverriderunv1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("cluster downloader validate create should not be registered, see NOTE in webhook/v1beta1/clusterdownloader_webhook.go", "name", obj.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *ocularcrashoverriderunv1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("cluster downloader validate update should not be registered, see NOTE in webhook/v1beta1/clusterdownloader_webhook.go", "name", newObj.GetName())

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ClusterDownloader.
func (v *ClusterDownloaderCustomValidator) ValidateDelete(ctx context.Context, obj *ocularcrashoverriderunv1beta1.ClusterDownloader) (admission.Warnings, error) {
	clusterdownloaderlog.Info("validation for cluster downloader upon deletion", "name", obj.GetName())
	return nil, v.checkForPipelinesReferencingClusterDownloader(ctx, obj)
}

func (v *ClusterDownloaderCustomValidator) checkForPipelinesReferencingClusterDownloader(ctx context.Context, downloader *ocularcrashoverriderunv1beta1.ClusterDownloader) error {
	pipelines := ocularcrashoverriderunv1beta1.PipelineList{}
	if err := v.c.List(ctx, &pipelines); err != nil {
		return fmt.Errorf("failed to list pipelines: %w", err)
	}
	var allErrs error
	for _, pipeline := range pipelines.Items {
		downloaderRef := pipeline.Spec.DownloaderRef
		if downloaderRef.Name == downloader.Name && downloaderRef.Kind == "ClusterDownloader" {
			allErrs = multierror.Append(allErrs, fmt.Errorf("this resource cannot be deleted because it is still referenced by 'Pipeline/%s in namespace %s'", pipeline.Name, pipeline.Namespace))
		}
	}

	if allErrs == nil {
		return nil
	}

	downloaderlog.Info("forbidden delete on cluster downloader", "name", downloader.Name, "error", allErrs)
	return apierrors.NewForbidden(
		schema.GroupResource{Group: "ocular.crashoverride.run", Resource: downloader.Name},
		downloader.Name, allErrs)
}
