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

	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/resources"
	"github.com/crashappsec/ocular/internal/validators"
)

// nolint:unused
// log is for logging in this package.
var pipelinelog = logf.Log.WithName("pipeline-resource")

// SetupPipelineWebhookWithManager registers the webhook for Pipeline in the manager.
func SetupPipelineWebhookWithManager(mgr ctrl.Manager) error {
	c := mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr, &ocularcrashoverriderunv1beta1.Pipeline{}).
		WithValidator(&PipelineCustomValidator{
			c: c,
		}).
		WithDefaulter(&PipelineCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-pipeline,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=mpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pipeline when those are created or updated.
type PipelineCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Pipeline.
func (d *PipelineCustomDefaulter) Default(_ context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) error {
	pipelinelog.Info("defaulting for Pipeline", "name", pipeline.GetName())

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

	pipeline.Spec.DownloaderRef = resources.ReferenceDefaulter(pipeline.Spec.DownloaderRef, "Downloader", pipeline.GetNamespace())
	pipeline.Spec.ProfileRef = resources.ReferenceDefaulter(pipeline.Spec.ProfileRef, "Profile", pipeline.GetNamespace())

	return nil
}

// NOTE: this validator is currently only enabled for 'create' and 'update'.
// additional options can be specified in the 'verbs' parameter
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-pipeline,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=vpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineCustomValidator struct is responsible for validating the Pipeline resource
// when it is created, updated, or deleted.
type PipelineCustomValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateCreate(ctx context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("validation for Pipeline upon create", "name", pipeline.GetName(), "validator", "create")
	return nil, validators.ValidatePipeline(ctx, v.c, pipeline)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateUpdate(ctx context.Context, _, newPipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("validation for Pipeline upon update", "name", newPipeline.GetName(), "validator", "update")

	// TODO(bthuilot): dont allow certain field updates
	// once the pipeline is running

	return nil, validators.ValidatePipeline(ctx, v.c, newPipeline)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pipeline.
func (v *PipelineCustomValidator) ValidateDelete(ctx context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("pipeline delete called but should not be registered, see NOTE in webhook/v1beta1/pipeline_webhook.go", "name", pipeline.GetName())

	return nil, nil
}
