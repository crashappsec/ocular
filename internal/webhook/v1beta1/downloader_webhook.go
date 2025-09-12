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
var downloaderlog = logf.Log.WithName("downloader-resource")

// SetupDownloaderWebhookWithManager registers the webhook for Downloader in the manager.
func SetupDownloaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Downloader{}).
		WithValidator(&DownloaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-downloader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=downloaders,verbs=delete,versions=v1beta1,name=vdownloader-v1beta1.kb.io,admissionReviewVersions=v1

// DownloaderCustomValidator struct is responsible for validating the Downloader resource
// when it is created, updated, or deleted.
type DownloaderCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &DownloaderCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	downloader, ok := obj.(*ocularcrashoverriderunv1beta1.Downloader)
	if !ok {
		return nil, fmt.Errorf("expected a Downloader object but got %T", obj)
	}

	downloaderlog.Info("downloader validate create should not be registered, see NOTE", "name", downloader.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	downloader, ok := newObj.(*ocularcrashoverriderunv1beta1.Downloader)
	if !ok {
		return nil, fmt.Errorf("expected a Downloader object for the newObj but got %T", newObj)
	}

	downloaderlog.Info("downloader validate update should not be registered, see NOTE", "name", downloader.GetName())

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	downloader, ok := obj.(*ocularcrashoverriderunv1beta1.Downloader)
	if !ok {
		return nil, fmt.Errorf("expected a Downloader object but got %T", obj)
	}

	var pipelines ocularcrashoverriderunv1beta1.PipelineList
	if err := v.c.List(ctx, &pipelines); err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}
	var merr *multierror.Error
	for _, pipeline := range pipelines.Items {
		namespace := pipeline.Spec.DownloaderRef.Namespace
		if namespace == "" {
			namespace = pipeline.Namespace
		}
		if pipeline.Spec.DownloaderRef.Name == downloader.Name && namespace == downloader.Namespace {
			downloaderlog.Info("found pipeline reference to downloader", "pipeline", pipeline.GetName(), "name", downloader.GetName())
			merr = multierror.Append(merr, fmt.Errorf("downloader %s is still referenced by pipeline %s", downloader.Name, pipeline.Name))
		}
	}

	return nil, merr.ErrorOrNil()
}
