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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var pipelinelog = logf.Log.WithName("pipeline-resource")

// SetupPipelineWebhookWithManager registers the webhook for Pipeline in the manager.
func SetupPipelineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ocularcrashoverriderunv1beta1.Pipeline{}).
		WithValidator(&PipelineCustomValidator{
			c: mgr.GetClient(),
		}).
		WithDefaulter(&PipelineCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-pipeline,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=mpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pipeline when those are created or updated.
type PipelineCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &PipelineCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Pipeline.
func (d *PipelineCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*ocularcrashoverriderunv1beta1.Pipeline)

	if !ok {
		return fmt.Errorf("expected an Pipeline object but got %T", obj)
	}

	if pipeline.Spec.UploadServiceAccountName == "" {
		pipeline.Spec.UploadServiceAccountName = "default"
	}
	if pipeline.Spec.ScanServiceAccountName == "" {
		pipeline.Spec.ScanServiceAccountName = "default"
	}

	if pipeline.Spec.TTLSecondsMaxLifetime == nil {
		pipeline.Spec.TTLSecondsMaxLifetime = ptr.To[int32](0)
	}

	pipeline.Status.Phase = ocularcrashoverriderunv1beta1.PipelinePending
	pipeline.Status.StageStatuses = ocularcrashoverriderunv1beta1.PipelineStageStatuses{
		DownloadStatus: ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
		ScanStatus:     ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
		UploadStatus:   ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-pipeline,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=vpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineCustomValidator struct is responsible for validating the Pipeline resource
// when it is created, updated, or deleted.
type PipelineCustomValidator struct {
	c client.Client
}

var _ webhook.CustomValidator = &PipelineCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pipeline, ok := obj.(*ocularcrashoverriderunv1beta1.Pipeline)
	if !ok {
		return nil, fmt.Errorf("expected a Pipeline object but got %T", obj)
	}

	return nil, validatePipeline(ctx, v.c, pipeline)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	pipeline, ok := newObj.(*ocularcrashoverriderunv1beta1.Pipeline)
	if !ok {
		return nil, fmt.Errorf("expected a Pipeline object for the newObj but got %T", newObj)
	}

	return nil, validatePipeline(ctx, v.c, pipeline)
}

func validatePipeline(ctx context.Context, c client.Client, pipeline *ocularcrashoverriderunv1beta1.Pipeline) error {
	var allErrs field.ErrorList
	if err := validatePipelineName(pipeline); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate Profile exists
	profileNamespace := pipeline.Spec.ProfileRef.Namespace
	if profileNamespace != "" && pipeline.Spec.ProfileRef.Namespace != pipeline.Namespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("profileRef").Child("namespace"), pipeline.Spec.ProfileRef.Namespace, "profileRef namespace must be empty or match the pipeline namespace"))
	}
	var profile ocularcrashoverriderunv1beta1.Profile
	err := c.Get(ctx, client.ObjectKey{Name: pipeline.Spec.ProfileRef.Name, Namespace: pipeline.Namespace}, &profile)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error fetching profile %s/%s: %w", pipeline.Spec.ProfileRef.Namespace, pipeline.Spec.ProfileRef.Name, err)
		}
		allErrs = append(allErrs, field.NotFound(field.NewPath("spec").Child("profileRef").Child("name"), fmt.Sprintf("%s/%s", pipeline.Spec.ProfileRef.Namespace, pipeline.Spec.ProfileRef.Name)))
	}

	// Validate Downloader exists
	downloaderNamespace := pipeline.Spec.DownloaderRef.Namespace
	if downloaderNamespace != "" && pipeline.Spec.DownloaderRef.Namespace != pipeline.Namespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("downloaderRef").Child("namespace"), pipeline.Spec.DownloaderRef.Namespace, "downloaderRef namespace must be empty or match the pipeline namespace"))
	}
	var downloader ocularcrashoverriderunv1beta1.Downloader
	err = c.Get(ctx, client.ObjectKey{Name: pipeline.Spec.DownloaderRef.Name, Namespace: pipeline.Namespace}, &downloader)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("error fetching dwonloader %s/%s: %w", pipeline.Spec.DownloaderRef.Namespace, pipeline.Spec.DownloaderRef.Name, err)
		}
		allErrs = append(allErrs, field.NotFound(field.NewPath("spec").Child("downloaderRef").Child("name"), fmt.Sprintf("%s/%s", pipeline.Spec.DownloaderRef.Namespace, pipeline.Spec.DownloaderRef.Name)))
	}

	var serviceAccount corev1.ServiceAccount
	err = c.Get(ctx, client.ObjectKey{Name: pipeline.Spec.ScanServiceAccountName, Namespace: pipeline.Namespace}, &serviceAccount)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error fetching scan service account %s: %w", pipeline.Spec.ScanServiceAccountName, err)
		}
		allErrs = append(allErrs, field.NotFound(field.NewPath("spec").Child("scanServiceAccountName"), pipeline.Spec.ScanServiceAccountName))
	}

	if len(profile.Spec.UploaderRefs) > 0 {
		var uploaderServiceAccount corev1.ServiceAccount
		err = c.Get(ctx, client.ObjectKey{Name: pipeline.Spec.UploadServiceAccountName, Namespace: pipeline.Namespace}, &uploaderServiceAccount)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error fetching uploader service account %s: %w", pipeline.Spec.UploadServiceAccountName, err)
			}
			allErrs = append(allErrs, field.NotFound(field.NewPath("spec").Child("uploadServiceAccountName"), pipeline.Spec.UploadServiceAccountName))
		}
	}

	if pipeline.Spec.TTLSecondsAfterFinished != nil && *pipeline.Spec.TTLSecondsAfterFinished < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("ttlSecondsAfterFinished"), pipeline.Spec.TTLSecondsAfterFinished, "must be non-negative"))
	}

	if pipeline.Spec.TTLSecondsMaxLifetime != nil && *pipeline.Spec.TTLSecondsMaxLifetime < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("ttlSecondsMax"), pipeline.Spec.TTLSecondsMaxLifetime, "must be non-negative"))
	}

	volumes := map[string]struct{}{}
	for _, vol := range append(downloader.Spec.Volumes, profile.Spec.Volumes...) {
		if _, exists := volumes[vol.Name]; exists {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("volumes").Child("name"), vol.Name))
		}
		volumes[vol.Name] = struct{}{}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "Pipeline"},
		pipeline.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pipeline, ok := obj.(*ocularcrashoverriderunv1beta1.Pipeline)
	if !ok {
		return nil, fmt.Errorf("expected a Pipeline object but got %T", obj)
	}

	pipelinelog.Info("pipeline delete called but should not be registered, see NOTE", "name", pipeline.GetName())

	return nil, nil
}

func validatePipelineName(pipeline *ocularcrashoverriderunv1beta1.Pipeline) *field.Error {
	if len(pipeline.Name) > validationutils.DNS1035LabelMaxLength-11 {
		// The service name length is 63 characters like all Kubernetes objects
		// (which must fit in a DNS subdomain). The pipeline controller appends
		// a 11-character suffix to the pipeline name (`-upload-svc`) when creating
		// a service for the uplodaers. The uplaoder service name length limit is 63 characters.
		// Therefore Pipeline names must have length <= 63-11=52. If we don't validate this here,
		// then service creation will fail later.
		return field.Invalid(field.NewPath("metadata").Child("name"), pipeline.Name, "must be no more than 52 characters")
	}
	return nil
}
