// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"context"
	"fmt"
	"time"

	errs "errors"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/resources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SearchReconciler reconciles a Search object
type SearchReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	SearchClusterRole string
}

// SetupWithManager sets up the controller with the Manager.
func (r *SearchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Search{}).
		Named("search").
		Owns(&batchv1.Job{}, builder.MatchEveryOwner).
		Complete(r)
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/finalizers,verbs=update
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers,verbs=get;list
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=watch;create;get;list;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=watch;create;get;list;update;patch;delete

func (r *SearchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the Pipeline instance to be reconciled
	search := &v1beta1.Search{}
	err := r.Get(ctx, req.NamespacedName, search)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // could have been deleted after reconcile request.
		}
		return ctrl.Result{}, err
	}

	// Handle finalizers (i.e. remove finalizer from resource if needed)
	finalized, err := resources.PerformFinalizer(ctx, search, "search.finalizers.ocular.crashoverride.run/cleanup", nil)
	if err != nil {
		l.Error(err, "error performing finalizer for pipeline", "name", search.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		l.Info("search has been finalized", "name", search.GetName(), "finalizers", search.GetFinalizers())
		if updateErr := r.Update(ctx, search); updateErr != nil {
			l.Error(updateErr, "failed to update after finalization", "name", search.GetName())
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// If the pipeline has a completion time, handle post-completion logic
	if search.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, search)
	}

	// Check referenced crawler exists and is valid
	crawler, exists, err := r.getCrawler(ctx, search)
	if err != nil {
		l.Error(err, "error fetching crawler for search", "name", search.GetName(), "crawlerRef", search.Spec.CrawlerRef)
		return ctrl.Result{}, err
	} else if !exists {
		l.Error(err, "crawler not found for search", "name", search.GetName(), "crawlerRef", search.Spec.CrawlerRef)
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "CrawlerNotFound",
				Message:            "The specified crawler does not exist.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, search, "step", "crawler not found")
	}

	if crawler.Status.Valid == nil || !*crawler.Status.Valid {
		l.Error(err, "crawler is not valid for pipeline", "name", search.GetName(), "crawlerRef", search.Spec.CrawlerRef)
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "CrawlerNotValid",
				Message:            "The specified crawler is not valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, search, "step", "crawler is invalid")
	}

	if err = v1beta1.ValidateCrawlerParameters(*crawler, search.Spec.Parameters); err != nil {
		l.Error(err, "error validating crawler parameters for search", "name", search.GetName())
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "InvalidParameters",
				Message:            fmt.Sprintf("The specified parameters are invalid: %v", err),
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, search, "step", "invalid crawler parameters")
	}

	envVars := generateBaseSearchEnvironment(search, crawler)

	containerOpts := generateBaseContainerOptions(envVars)

	copyOwnership := func(_ context.Context, _ client.Client, actual, desired *corev1.ServiceAccount) error {
		desired.OwnerReferences = actual.OwnerReferences
		return nil
	}

	// generate desired upload job, service, and scan job
	searchServiceAccount := r.newSearchServiceAccount(search)

	// here
	if err = resources.ReconcileChildResource[*corev1.ServiceAccount](ctx, r.Client, searchServiceAccount, nil, r.Scheme, copyOwnership); err != nil {
		l.Error(err, "error reconciling service account for search", "name", search.GetName())
		return ctrl.Result{}, err
	}

	searchRoleBinding := r.newSearchRoleBinding(search, searchServiceAccount)

	if err = resources.ReconcileChildResource[*corev1.ServiceAccount](ctx, r.Client, searchRoleBinding, searchServiceAccount, r.Scheme, nil); err != nil {
		l.Error(err, "error reconciling upload job for pipeline", "name", search.GetName())
		return ctrl.Result{}, err
	}

	searchJob := r.newSearchJob(search, crawler, searchServiceAccount, containerOpts...)

	// helper function to copy status from existing job if found
	copyStatus := func(_ context.Context, _ client.Client, src *batchv1.Job, dest *batchv1.Job) error {
		dest.Status = src.Status
		dest.ObjectMeta = src.ObjectMeta
		return nil
	}

	if err = resources.ReconcileChildResource[*batchv1.Job](ctx, r.Client, searchJob, search, r.Scheme, copyStatus); err != nil {
		l.Error(err, "error reconciling upload job for pipeline", "name", search.GetName())
		return ctrl.Result{}, err
	}
	search.Status.SearchJob = &corev1.ObjectReference{
		Name:      searchJob.Name,
		Namespace: searchJob.Namespace,
	}

	// Update status to reflect jobs have been created
	if search.Status.StartTime == nil {
		startTime := metav1.NewTime(time.Now())
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "SearchJobCreated",
				Message:            fmt.Sprintf("The scan job %s has been created.", searchJob.Name),
				LastTransitionTime: startTime,
			},
		}
		search.Status.StartTime = &startTime
		if err := updateStatus(ctx, r.Client, search, "step", "child resources created"); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err = r.ensureJobOwnsServiceAccount(ctx, searchJob, searchServiceAccount); err != nil {
		l.Error(err, "error ensuring job owns service account", "name", search.GetName())
		return ctrl.Result{}, err
	}

	// Check for completion of jobs and update status accordingly
	return r.handleCompletion(ctx, search, searchJob)
}

func (r *SearchReconciler) getCrawler(ctx context.Context, search *v1beta1.Search) (*v1beta1.Crawler, bool, error) {
	crawler := &v1beta1.Crawler{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      search.Spec.CrawlerRef,
		Namespace: search.Namespace,
	}, crawler)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return crawler, true, nil
}

func (r *SearchReconciler) newSearchServiceAccount(search *v1beta1.Search) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("search-%s-sa", search.GetName()),
			Namespace: search.GetNamespace(),
			Labels: map[string]string{
				"app":        "ocular",
				"component":  "search",
				"searchName": search.GetName(),
			},
		},
	}
}

func (r *SearchReconciler) newSearchRoleBinding(search *v1beta1.Search, sa *corev1.ServiceAccount) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("search-%s-rb", search.GetName()),
			Namespace: search.GetNamespace(),
			Labels: map[string]string{
				"app":        "ocular",
				"component":  "search",
				"searchName": search.GetName(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     r.SearchClusterRole,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.GetName(),
				Namespace: sa.GetNamespace(),
			},
		},
	}
}

func (r *SearchReconciler) newSearchJob(search *v1beta1.Search, crawler *v1beta1.Crawler, sa *corev1.ServiceAccount, containerOpts ...resources.ContainerOption) *batchv1.Job {
	labels := map[string]string{
		"app":        "ocular",
		"component":  "search",
		"searchName": search.GetName(),
	}

	var envVars []corev1.EnvVar
	for paramName, paramDef := range crawler.Spec.Parameters {
		paramValue, paramSet := search.Spec.Parameters[paramName]
		envVarName := v1beta1.ParameterToEnvironmentVariable(paramName)
		if paramSet {
			envVars = append(envVars, corev1.EnvVar{
				Name:  envVarName,
				Value: paramValue,
			})
		} else {
			// if not set, and default exists, use default
			if paramDef.Default != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name:  envVarName,
					Value: paramDef.Default,
				})
			}
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("search-%s-job", search.GetName()),
			Namespace: search.GetNamespace(),
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: sa.GetName(),
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: resources.ApplyOptionsToContainers([]corev1.Container{
						crawler.Spec.Container,
					}, append(containerOpts, resources.ContainerWithAdditionalEnvVars(envVars...))...),
					Volumes: crawler.Spec.Volumes,
				},
			},
			BackoffLimit: ptr.To[int32](0),
		},
	}

	return job
}

func (r *SearchReconciler) ensureJobOwnsServiceAccount(ctx context.Context, job *batchv1.Job, sa *corev1.ServiceAccount) error {
	err := ctrl.SetControllerReference(job, sa, r.Scheme)
	// ensure the job owns the service account
	if err != nil {
		alreadyOwnedErr := &controllerutil.AlreadyOwnedError{}
		if errs.As(err, &alreadyOwnedErr) {
			return nil
		}
		return fmt.Errorf("failed to set controller reference for service account: %w", err)
	}
	return r.Update(ctx, sa)
}

func (r *SearchReconciler) handleCompletion(ctx context.Context, search *v1beta1.Search, job *batchv1.Job) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// job has completed but we haven't recorded it yet
	if search.Status.CompletionTime == nil && job.Status.CompletionTime != nil {
		l.Info("search job completed, recording completion in status", "name", search.GetName(), "job", job.GetName())
		failed := job.Status.Failed > 0
		var (
			status  = metav1.ConditionTrue
			reason  = "SearchJobCompleted"
			message = "Search has completed successfully."
		)
		search.Status.Failed = ptr.To(failed)
		if failed {
			status = metav1.ConditionFalse
			reason = "SearchJobFailed"
			message = "Search reported a container failure."
		}
		completionTime := metav1.NewTime(time.Now())
		search.Status.CompletionTime = &completionTime
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               "Completed",
				Status:             status,
				Reason:             reason,
				Message:            message,
				LastTransitionTime: completionTime,
			},
		}
		if err := updateStatus(ctx, r.Client, search, "step", "search job completion"); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func generateBaseSearchEnvironment(_ *v1beta1.Search, crawler *v1beta1.Crawler) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:      v1beta1.EnvVarSearchName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		},
		{
			Name:  v1beta1.EnvVarCrawlerName,
			Value: crawler.GetName(),
		},
		{
			Name:      v1beta1.EnvVarNamespaceName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
		},
	}
}

func (r *SearchReconciler) handlePostCompletion(ctx context.Context, search *v1beta1.Search) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	if search.Spec.TTLSecondsAfterFinished != nil {
		// check if we need to delete the search
		finishTime := search.Status.CompletionTime.Time
		ttl := time.Duration(*search.Spec.TTLSecondsAfterFinished) * time.Second
		deleteTime := finishTime.Add(ttl)
		if time.Now().After(deleteTime) {
			l.Info("search has exceeded its TTL, deleting", "name", search.GetName(), "completionTime", search.Status.CompletionTime, "ttlSecondsAfterFinished", *search.Spec.TTLSecondsAfterFinished)
			if err := r.Delete(ctx, search); err != nil {
				l.Error(err, "error deleting search after TTL exceeded", "name", search.GetName())
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else {
			l.Info("search has completed, checking TTL before next reconciliation", "name", search.GetName(), "completionTime", search.Status.CompletionTime, "ttlSecondsAfterFinished", *search.Spec.TTLSecondsAfterFinished)
			return ctrl.Result{RequeueAfter: time.Until(deleteTime)}, nil
		}
	}
	l.Info("search has completed, skipping reconciliation", "name", search.GetName(), "completionTime", search.Status.CompletionTime)
	return ctrl.Result{}, nil
}
