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
var downloaderlog = logf.Log.WithName("downloader-resource")

// SetupDownloaderWebhookWithManager registers the webhook for Downloader in the manager.
func SetupDownloaderWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Downloader{}).
		WithValidator(&DownloaderCustomValidator{
			c: mgr.GetClient(),
		}).
		Complete()
}

// NOTE: currently the downloader is only configured to run as a validating webhook
// during the deletion of a Downloader resource to validate that no Pipeline resources
// are still referencing it. Creation and update validation is not currently needed
// because the controller reconciles the Downloader references in the Pipeline resources
// and will set the status to indicate if a reference is invalid. If in the future
// there is a need to validate Downloader resources on creation or update, the
// ValidateCreate and ValidateUpdate methods below can be implemented and 'create;update'
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-downloader,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=downloaders,verbs=delete,versions=v1beta1,name=vdownloader-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

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

	downloaderlog.Info("downloader validate create should not be registered, see NOTE in webhook/v1beta1/downloader_webhook.go", "name", downloader.GetName())

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	downloader, ok := newObj.(*ocularcrashoverriderunv1beta1.Downloader)
	if !ok {
		return nil, fmt.Errorf("expected a Downloader object for the newObj but got %T", newObj)
	}

	downloaderlog.Info("downloader validate update should not be registered, see NOTE in webhook/v1beta1/downloader_webhook.go", "name", downloader.GetName())

	return nil, nil
}

func (v *DownloaderCustomValidator) checkForPipelinesReferencingDownloader(ctx context.Context, downloader *ocularcrashoverriderunv1beta1.Downloader) error {
	var pipelines ocularcrashoverriderunv1beta1.PipelineList
	if err := v.c.List(ctx, &pipelines); err != nil {
		return fmt.Errorf("failed to list pipelines: %w", err)
	}
	var allErrs field.ErrorList
	for _, pipeline := range pipelines.Items {
		namespace := pipeline.Spec.DownloaderRef.Namespace
		if namespace == "" {
			namespace = pipeline.Namespace
		}
		if pipeline.Spec.DownloaderRef.Name == downloader.Name && namespace == downloader.Namespace {
			downloaderlog.Info("found pipeline reference to downloader", "pipeline", pipeline.GetName(), "name", downloader.GetName())
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), downloader.Name, "cannot be deleted because it is still referenced by a Pipeline resource"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Downloader"},
		downloader.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Downloader.
func (v *DownloaderCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	downloader, ok := obj.(*ocularcrashoverriderunv1beta1.Downloader)
	if !ok {
		return nil, fmt.Errorf("expected a Downloader object but got %T", obj)
	}

	return nil, v.checkForPipelinesReferencingDownloader(ctx, downloader)
}
