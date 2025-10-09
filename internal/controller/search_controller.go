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
	ocuarlRuntime "github.com/crashappsec/ocular/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
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
		Owns(&corev1.Pod{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/finalizers,verbs=update
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts;pods,verbs=watch;create;get;list;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=watch;create;get;list;update;patch;delete

func (r *SearchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the Pipeline instance to be reconciled
	search := &v1beta1.Search{}
	err := r.Get(ctx, req.NamespacedName, search)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the pipeline has a completion time, handle post-completion logic
	if search.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, search)
	}

	// Check referenced crawler exists and is valid
	crawler := &v1beta1.Crawler{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      search.Spec.CrawlerRef.Name,
		Namespace: search.Namespace,
	}, crawler)
	if err != nil {
		return ctrl.Result{}, err
	}

	envVars := generateBaseSearchEnvironment(search, crawler)

	containerOpts := generateBaseContainerOptions(envVars)

	copyOwnership := func(_ context.Context, _ client.Client, actual, desired *corev1.ServiceAccount) error {
		desired.OwnerReferences = actual.OwnerReferences
		return nil
	}

	// generate desired upload pod, service, and scan pod
	searchServiceAccount := r.newSearchServiceAccount(search)

	if err = resources.ReconcileChildResource[*corev1.ServiceAccount](ctx, r.Client, searchServiceAccount, nil, r.Scheme, copyOwnership); err != nil {
		l.Error(err, "error reconciling service account for search", "name", search.GetName())
		return ctrl.Result{}, err
	}

	searchRoleBinding := r.newSearchRoleBinding(search, searchServiceAccount)

	if err = resources.ReconcileChildResource[*rbacv1.RoleBinding](ctx, r.Client, searchRoleBinding, searchServiceAccount, r.Scheme, nil); err != nil {
		l.Error(err, "error reconciling upload pod for pipeline", "name", search.GetName())
		return ctrl.Result{}, err
	}

	searchPod := r.newSearchPod(search, crawler, searchServiceAccount, containerOpts...)

	searchPod, err = reconcilePodFromLabel(ctx, r.Client, r.Scheme, search, searchPod, map[string]string{
		SearchLabelKey: search.GetName(),
		TypeLabelKey:   PodTypeSearch,
	})
	if err != nil {
		l.Error(err, "error reconciling upload pod for search", "name", search.GetName())
		return ctrl.Result{}, err
	}

	// Update status to reflect pods have been created
	if search.Status.StartTime == nil {
		startTime := metav1.NewTime(time.Now())
		search.Status.Conditions = []metav1.Condition{
			{
				Type:               v1beta1.StartedConditionType,
				Status:             metav1.ConditionTrue,
				Reason:             "SearchPodCreated",
				Message:            fmt.Sprintf("The scan pod %s has been created.", searchPod.Name),
				LastTransitionTime: startTime,
			},
		}
		search.Status.StartTime = &startTime
		if err := updateStatus(ctx, r.Client, search, "step", "child resources created"); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err = r.ensurePodOwnsServiceAccount(ctx, searchPod, searchServiceAccount); err != nil {
		l.Error(err, "error ensuring pod owns service account", "name", search.GetName())
		return ctrl.Result{}, err
	}

	// Check for completion of pods and update status accordingly
	return r.handleCompletion(ctx, search, searchPod)
}

func (r *SearchReconciler) newSearchServiceAccount(search *v1beta1.Search) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      search.GetName() + searchResourceSuffix,
			Namespace: search.GetNamespace(),
			Labels: map[string]string{
				TypeLabelKey:   ServiceTypeSearch,
				SearchLabelKey: search.GetName(),
			},
		},
	}
}

func (r *SearchReconciler) newSearchRoleBinding(search *v1beta1.Search, sa *corev1.ServiceAccount) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      search.GetName() + searchResourceSuffix,
			Namespace: search.GetNamespace(),
			Labels: map[string]string{
				SearchLabelKey: search.GetName(),
				TypeLabelKey:   RoleBindingTypeSearch,
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

func (r *SearchReconciler) newSearchPod(search *v1beta1.Search, crawler *v1beta1.Crawler, sa *corev1.ServiceAccount, containerOpts ...resources.ContainerOption) *corev1.Pod {
	envVars := make([]corev1.EnvVar, 0, len(crawler.Spec.Parameters))

	// this loop does not check for duplicate parameters NOR
	// required parameters to be set. This is done during
	// profile admission validation.
	var setParams = map[string]struct{}{}
	for _, paramDef := range search.Spec.CrawlerRef.Parameters {
		setParams[paramDef.Name] = struct{}{}
		envVarName := ocuarlRuntime.ParameterToEnvironmentVariable(paramDef.Name)
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarName,
			Value: paramDef.Value,
		})
	}

	for _, paramDef := range crawler.Spec.Parameters {
		if _, exists := setParams[paramDef.Name]; !exists {
			if paramDef.Default != nil {
				envVars = append(envVars, corev1.EnvVar{
					Name:  ocuarlRuntime.ParameterToEnvironmentVariable(paramDef.Name),
					Value: *paramDef.Default,
				})
			}
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: search.GetName() + "-",
			Namespace:    search.GetNamespace(),
			Labels: map[string]string{
				SearchLabelKey:  search.GetName(),
				CrawlerLabelKey: crawler.GetName(),
				TypeLabelKey:    PodTypeSearch,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: sa.GetName(),
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: resources.ApplyOptionsToContainers(
				[]corev1.Container{crawler.Spec.Container},
				append(containerOpts, resources.ContainerWithAdditionalEnvVars(envVars...))...),
			Volumes: crawler.Spec.Volumes,
		},
	}

	return pod
}

func (r *SearchReconciler) ensurePodOwnsServiceAccount(ctx context.Context, pod *corev1.Pod, sa *corev1.ServiceAccount) error {
	err := ctrl.SetControllerReference(pod, sa, r.Scheme)
	// ensure the pod owns the service account
	if err != nil {
		alreadyOwnedErr := &controllerutil.AlreadyOwnedError{}
		if errs.As(err, &alreadyOwnedErr) {
			return nil
		}
		return fmt.Errorf("failed to set controller reference for service account: %w", err)
	}
	return r.Update(ctx, sa)
}

func (r *SearchReconciler) handleCompletion(ctx context.Context, search *v1beta1.Search, pod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	switch pod.Status.Phase {
	case corev1.PodFailed:
		l.Info("search pod has failed", "name", search.GetName(), "pod", pod.GetName())
		if search.Status.CompletionTime == nil {
			t := metav1.NewTime(time.Now())
			search.Status.CompletionTime = ptr.To(t)
			search.Status.Conditions = append(search.Status.Conditions, metav1.Condition{
				Type:               v1beta1.CompletedSuccessfullyConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "SearchJobFailed",
				Message:            "Search reported a container failure.",
				LastTransitionTime: t,
			})
		}
	case corev1.PodSucceeded:
		l.Info("search pod has succeeded", "name", search.GetName(), "pod", pod.GetName())
		if search.Status.CompletionTime == nil {
			t := metav1.NewTime(time.Now())
			search.Status.CompletionTime = ptr.To(t)
			search.Status.Conditions = append(search.Status.Conditions, metav1.Condition{
				Type:               v1beta1.CompletedSuccessfullyConditionType,
				Status:             metav1.ConditionTrue,
				Reason:             "SearchPodSucceeded",
				Message:            "Search completed successfully.",
				LastTransitionTime: t,
			})
		}
	case corev1.PodPending, corev1.PodRunning:
		l.Info("search pod is still running", "name", search.GetName(), "pod", pod.GetName(), "phase", pod.Status.Phase)
		return ctrl.Result{}, nil
	default:
		l.Info("search pod is in unknown state, requeuing", "name", search.GetName(), "pod", pod.GetName(), "phase", pod.Status.Phase)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, updateStatus(ctx, r.Client, search, "step", "search pod completion")
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
