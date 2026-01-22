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

	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var cronsearchlog = logf.Log.WithName("cronsearch-resource")

// SetupCronSearchWebhookWithManager registers the webhook for CronSearch in the manager.
func SetupCronSearchWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &v1beta1.CronSearch{}).
		WithValidator(&CronSearchCustomValidator{}).
		WithDefaulter(&CronSearchCustomDefaulter{
			DefaultConcurrencyPolicy:          v1beta1.AllowConcurrent,
			DefaultSuspend:                    false,
			DefaultSuccessfulJobsHistoryLimit: 3,
			DefaultFailedJobsHistoryLimit:     1,
		}).
		Complete()
}

/* Most of the code below was adapted from the
   kubebuilder tutorial implementing a CronJob webhook:
   https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/internal/webhook/v1/cronjob_webhook.go
*/

// +kubebuilder:webhook:path=/mutate-ocular-crashoverride-run-v1beta1-cronsearch,mutating=true,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=cronsearches,verbs=create;update,versions=v1beta1,name=mcronsearch-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// CronSearchCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind CronSearch when those are created or updated.
type CronSearchCustomDefaulter struct {
	// Default values for various CronJob fields
	DefaultConcurrencyPolicy          v1beta1.ConcurrencyPolicy
	DefaultSuspend                    bool
	DefaultSuccessfulJobsHistoryLimit int32
	DefaultFailedJobsHistoryLimit     int32
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Search.
func (d *CronSearchCustomDefaulter) Default(_ context.Context, cronSearch *v1beta1.CronSearch) error {
	cronsearchlog.Info("defaulting for CronSearch", "name", cronSearch.GetName())

	d.applyDefaults(cronSearch)
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-cronsearch,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=cronsearches,verbs=create;update,versions=v1beta1,name=vcronsearch-v1beta1.kb.io,admissionReviewVersions=v1
func (d *CronSearchCustomDefaulter) applyDefaults(cronJob *v1beta1.CronSearch) {
	if cronJob.Spec.ConcurrencyPolicy == "" {
		cronJob.Spec.ConcurrencyPolicy = d.DefaultConcurrencyPolicy
	}
	if cronJob.Spec.Suspend == nil {
		cronJob.Spec.Suspend = new(bool)
		*cronJob.Spec.Suspend = d.DefaultSuspend
	}
	if cronJob.Spec.SuccessfulJobsHistoryLimit == nil {
		cronJob.Spec.SuccessfulJobsHistoryLimit = new(int32)
		*cronJob.Spec.SuccessfulJobsHistoryLimit = d.DefaultSuccessfulJobsHistoryLimit
	}
	if cronJob.Spec.FailedJobsHistoryLimit == nil {
		cronJob.Spec.FailedJobsHistoryLimit = new(int32)
		*cronJob.Spec.FailedJobsHistoryLimit = d.DefaultFailedJobsHistoryLimit
	}
}

// NOTE: currently the cronsearch is only configured to run as a validating webhook
// during the update and/or creation of a CronSearch resource to validate that
// 1) the schedule is a valid cron format, and
// 2) the name is not too long (so that the jobs it creates do not exceed k8s name limits).
// Deletion is currently not needed since there are no special validations needed
// when a CronSearch is deleted. If in the future there is a need to validate
// CronSearch resources on deletion, the ValidateDelete method below can be implemented and 'delete'
// can be added to the verbs in the kubebuilder marker below.
// +kubebuilder:webhook:path=/validate-ocular-crashoverride-run-v1beta1-cronsearch,mutating=false,failurePolicy=fail,sideEffects=None,groups=ocular.crashoverride.run,resources=cronsearches,verbs=create;update,versions=v1beta1,name=vcronsearch-v1beta1.ocular.crashoverride.run,admissionReviewVersions=v1

// CronSearchCustomValidator struct is responsible for validating the CronSearch resource
// when it is created, updated, or deleted.
type CronSearchCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type CronSearch.
func (v *CronSearchCustomValidator) ValidateCreate(_ context.Context, cronSearch *v1beta1.CronSearch) (admission.Warnings, error) {
	cronsearchlog.Info("validating CronSearch creation", "name", cronSearch.GetName())

	return nil, validateCronSearch(cronSearch)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type CronSearch.
func (v *CronSearchCustomValidator) ValidateUpdate(_ context.Context, _, newCronSearch *v1beta1.CronSearch) (admission.Warnings, error) {
	cronsearchlog.Info("validating CronSearch update", "name", newCronSearch.GetName())

	return nil, validateCronSearch(newCronSearch)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type CronSearch.
func (v *CronSearchCustomValidator) ValidateDelete(_ context.Context, cronSearch *v1beta1.CronSearch) (admission.Warnings, error) {

	cronsearchlog.Info("cronsearch validate update should not be registered, see NOTE in webhook/v1beta1/cronsearch_webhook.go", "name", cronSearch.GetName())

	return nil, nil
}

// validateCronSearch validates the fields of a v1beta1.CronSearch object.
func validateCronSearch(cronSearch *v1beta1.CronSearch) error {
	var allErrs field.ErrorList
	if err := validateCronSearchName(cronSearch); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateCronSearchSpec(cronSearch); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ocular.crashoverride.run", Kind: "CronSearch"},
		cronSearch.Name, allErrs)
}

func validateCronSearchSpec(cronSearch *v1beta1.CronSearch) *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	return validateScheduleFormat(
		cronSearch.Spec.Schedule,
		field.NewPath("spec").Child("schedule"))
}

func validateScheduleFormat(schedule string, fldPath *field.Path) *field.Error {
	if _, err := cron.ParseStandard(schedule); err != nil {
		return field.Invalid(fldPath, schedule, err.Error())
	}
	return nil
}

func validateCronSearchName(cronSearch *v1beta1.CronSearch) *field.Error {
	if len(cronSearch.Name) > validationutils.DNS1035LabelMaxLength-11 {
		// The job name length is 63 characters like all Kubernetes objects
		// (which must fit in a DNS subdomain). The cronSearch controller appends
		// a 11-character suffix to the cronjob (`-$TIMESTAMP`) when creating
		// a job. The job name length limit is 63 characters. Therefore cronSearch
		// names must have length <= 63-11=52. If we don't validate this here,
		// then job creation will fail later.
		return field.Invalid(field.NewPath("metadata").Child("name"), cronSearch.Name, "must be no more than 52 characters")
	}
	return nil
}
