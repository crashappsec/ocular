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
		WithValidator(&PipelineValidator{
			c: c,
		}).
		WithDefaulter(&PipelineDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-pipeline,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=mpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pipeline when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PipelineDefaulter struct{}

// Default implements webhook.Defaulter so a webhook will be registered for the Kind Pipeline.
func (d *PipelineDefaulter) Default(_ context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) error {
	pipelinelog.Info("defaulting for Pipeline", "name", pipeline.GetName())

	if pipeline.Spec.ServiceAccountName == "" {
		pipeline.Spec.ServiceAccountName = "default"
	}

	pipeline.Status.Phase = ocularcrashoverriderunv1beta1.PipelinePending
	pipeline.Status.StageStatuses = ocularcrashoverriderunv1beta1.PipelineStageStatuses{
		DownloadStatus: ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
		ScanStatus:     ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
		UploadStatus:   ocularcrashoverriderunv1beta1.PipelineStageNotStarted,
	}

	pipeline.Spec.DownloaderRef = resources.ReferenceDefaulter(pipeline.Spec.DownloaderRef, "Downloader")
	pipeline.Spec.ProfileRef = resources.ReferenceDefaulter(pipeline.Spec.ProfileRef, "Profile")

	return nil
}

// NOTE: this validator is currently only enabled for 'create' and 'update'.
// additional options can be specified in the 'verbs' parameter
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-pipeline,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=pipelines,verbs=create;update,versions=v1beta1,name=vpipeline-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// PipelineValidator struct is responsible for validating the Pipeline resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PipelineValidator struct {
	c client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type Pipeline.
func (v *PipelineValidator) ValidateCreate(ctx context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("validation for Pipeline upon create", "name", pipeline.GetName(), "validator", "create")
	return nil, validators.ValidatePipeline(ctx, v.c, pipeline)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type Pipeline.
func (v *PipelineValidator) ValidateUpdate(ctx context.Context, _, newPipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("validation for Pipeline upon update", "name", newPipeline.GetName(), "validator", "update")

	// TODO(bthuilot): dont allow certain field updates
	// once the pipeline is running

	return nil, validators.ValidatePipeline(ctx, v.c, newPipeline)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type Pipeline.
func (v *PipelineValidator) ValidateDelete(ctx context.Context, pipeline *ocularcrashoverriderunv1beta1.Pipeline) (admission.Warnings, error) {
	pipelinelog.Info("pipeline delete called but should not be registered, see NOTE in webhook/v1beta1/pipeline_webhook.go", "name", pipeline.GetName())

	return nil, nil
}
