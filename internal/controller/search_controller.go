// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/containers"
	"github.com/crashappsec/ocular/internal/resources"
	ocularRuntime "github.com/crashappsec/ocular/pkg/runtime"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	searchPodsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ocular_search_pods_created_total",
			Help: "Number of search pods ocular has created",
		},
		[]string{"crawler", "namespace"},
	)
	searchDurationSeconds = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ocular_search_duration_seconds",
			Help: "Number of seconds it took the ocular search to complete",
		},
		[]string{"crawler", "namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		searchPodsCreated,
		searchDurationSeconds,
	)
}

// SearchReconciler reconciles a Search object
type SearchReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	SearchClusterRole string
	SidecarImage      string
	SidecarPullPolicy corev1.PullPolicy
}

// SetupWithManager sets up the controller with the Manager.
func (r *SearchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Search{}).
		Named("search").
		Owns(&corev1.Pod{}).
		Complete(r)
}

const (
	pipelineTemplateFile = "template.json"
	pipelineTemplateDir  = "/etc/ocular"
	pipelineTemplatePath = pipelineTemplateDir + "/" + pipelineTemplateFile

	ocularFIFODir       = "/run/ocular"
	pipelineFIFOFile    = "targets.fifo"
	pipelineFIFOPath    = ocularFIFODir + "/" + pipelineFIFOFile
	searchFIFOFile      = "crawlers.fifo"
	searchFIFOPath      = ocularFIFODir + "/" + searchFIFOFile
	sidecarCompletePath = ocularFIFODir + "/" + "complete"
)

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=searches/finalizers,verbs=update
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=crawlers;clustercrawlers,verbs=get;list;watch
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

	crawlerName := search.Spec.CrawlerRef.Name
	crawlerSpec, err := resources.CrawlerSpecFromReference(ctx, r.Client, req.Namespace, search.Spec.CrawlerRef.ObjectReference)
	if err != nil {
		return ctrl.Result{}, err
	}

	metricLabels := prometheus.Labels{"namespace": search.Namespace, "crawler": crawlerName}

	envVars := generateBaseSearchEnvironment(search, crawlerName)

	containerOpts := generateBaseContainerOptions(envVars)

	copyOwnership := func(_ context.Context, _ client.Client, actual, desired *corev1.ServiceAccount) error {
		desired.OwnerReferences = actual.OwnerReferences
		return nil
	}

	// generate desired upload pod, service, and scan pod
	var searchServiceAccount *corev1.ServiceAccount
	if search.Spec.ServiceAccountNameOverride != "" {
		searchServiceAccount, err = r.getServiceAccountFromOverride(ctx, search)
		if err != nil {
			l.Error(err, "error getting service account from override for search",
				"name", search.GetName(), "serviceaccount", search.Spec.ServiceAccountNameOverride)
			return ctrl.Result{}, err
		}
	} else {
		searchServiceAccount = r.newSearchServiceAccount(search)
		if err = reconcileChildResource(ctx, r.Client, searchServiceAccount, search, r.Scheme, copyOwnership); err != nil {
			l.Error(err, "error reconciling service account for search", "name", search.GetName())
			return ctrl.Result{}, err
		}
		search.Spec.ServiceAccountNameOverride = searchServiceAccount.Name
		if err = r.Update(ctx, search); err != nil {
			l.Error(err, "error updating search with service account name override", "name", search.GetName())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	searchRoleBinding := r.newSearchRoleBinding(search, searchServiceAccount)

	if err = reconcileChildResource[*rbacv1.RoleBinding](ctx, r.Client, searchRoleBinding, search, r.Scheme, nil); err != nil {
		l.Error(err, "error reconciling upload pod for pipeline", "name", search.GetName())
		return ctrl.Result{}, err
	}

	searchPod, err := r.newSearchPod(search, crawlerSpec, searchServiceAccount, containerOpts...)
	if err != nil {
		l.Error(err, "failed to generate search pod", "search", search.Name, "crawler", crawlerName)
		return ctrl.Result{}, err
	}

	searchPod, err = reconcilePodFromLabel(ctx, r.Client, r.Scheme, search, searchPod, []string{
		v1beta1.SearchLabelKey,
		v1beta1.TypeLabelKey,
	}, searchPodsCreated.With(metricLabels))
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

	// Check for completion of pods and update status accordingly
	return r.handleCompletion(ctx, search, searchPod)
}

func (r *SearchReconciler) newSearchServiceAccount(search *v1beta1.Search) *corev1.ServiceAccount {

	labels := map[string]string{
		v1beta1.SearchLabelKey: search.GetName(),
		v1beta1.TypeLabelKey:   v1beta1.ServiceAccountTypeSearch,
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      search.GetName() + searchResourceSuffix,
			Namespace: search.GetNamespace(),
			Labels:    labels,
		},
	}
}

func (r *SearchReconciler) getServiceAccountFromOverride(ctx context.Context, search *v1beta1.Search) (*corev1.ServiceAccount, error) {
	var sa corev1.ServiceAccount
	err := r.Get(ctx, client.ObjectKey{
		Name:      search.Spec.ServiceAccountNameOverride,
		Namespace: search.Namespace,
	}, &sa)
	return &sa, err

}

func (r *SearchReconciler) newSearchRoleBinding(search *v1beta1.Search, sa *corev1.ServiceAccount) *rbacv1.RoleBinding {

	labels := map[string]string{
		v1beta1.SearchLabelKey: search.GetName(),
		v1beta1.TypeLabelKey:   v1beta1.RoleBindingTypeSearch,
	}

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      search.GetName() + searchResourceSuffix,
			Namespace: search.GetNamespace(),
			Labels:    labels,
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

func (r *SearchReconciler) newSearchPod(search *v1beta1.Search, crawlerSpec v1beta1.CrawlerSpec, sa *corev1.ServiceAccount, containerOpts ...containers.Option) (*corev1.Pod, error) {
	envVars := make([]corev1.EnvVar, 0, len(crawlerSpec.Parameters))

	// this loop does not check for duplicate parameters NOR
	// required parameters to be set. This is done during
	// profile admission validation.
	var setParams = map[string]struct{}{}
	for _, paramDef := range search.Spec.CrawlerRef.Parameters {
		setParams[paramDef.Name] = struct{}{}
		envVarName := ocularRuntime.ParameterToEnvironmentVariable(paramDef.Name)
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarName,
			Value: paramDef.Value,
		})
	}

	for _, paramDef := range crawlerSpec.Parameters {
		if _, exists := setParams[paramDef.Name]; !exists {
			if paramDef.Default != nil {
				envVars = append(envVars, corev1.EnvVar{
					Name:  ocularRuntime.ParameterToEnvironmentVariable(paramDef.Name),
					Value: *paramDef.Default,
				})
			}
		}
	}

	templateVolume := corev1.Volume{
		Name: "pipeline-template",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: pipelineTemplateFile,
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.annotations['" + v1beta1.PipelineTemplateAnnotation + "']",
						},
					},
				},
			},
		},
	}
	socketVolume := corev1.Volume{
		Name: "pipeline-socket",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}

	pipelineTemplateJSON, err := json.Marshal(search.Spec.Scheduler.PipelineTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal pipeline template: %w", err)
	}

	labels := generateChildLabels(search)
	labels[v1beta1.SearchLabelKey] = search.GetName()
	labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeSearch
	labels[v1beta1.CrawlerLabelKey] = search.Spec.CrawlerRef.Name
	annotations := make(map[string]string)
	annotations[v1beta1.PipelineTemplateAnnotation] = string(pipelineTemplateJSON)

	schedulerSidecarContainer := r.generateSchedulerSidecarContainer(search)
	keepaliveSidecarContainer := r.generateKeepaliveSidecarContainer(search)
	initSidecarContainer := r.generateInitSidecarContainer(search)

	containerOpts = append(containerOpts,
		containers.WithAdditionalEnvVars(envVars...),
		containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
			Name:      templateVolume.Name,
			MountPath: pipelineTemplateDir,
		}, corev1.VolumeMount{
			Name:      socketVolume.Name,
			MountPath: ocularFIFODir,
		}))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: search.GetName() + "-",
			Namespace:    search.GetNamespace(),
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: sa.GetName(),
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers:     containers.ApplyOptions([]corev1.Container{schedulerSidecarContainer, initSidecarContainer, crawlerSpec.Container}, containerOpts...),
			Containers:         containers.ApplyOptions([]corev1.Container{keepaliveSidecarContainer}, containerOpts...),
			Volumes:            append(crawlerSpec.Volumes, templateVolume, socketVolume),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: ptr.To(int64(0)),
			},
		},
	}

	return pod, nil
}

func (r *SearchReconciler) handleCompletion(ctx context.Context, search *v1beta1.Search, pod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	metricLabels := prometheus.Labels{"namespace": search.Namespace, "crawler": search.Spec.CrawlerRef.Name}
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
	if err := updateStatus(ctx, r.Client, search, "step", "search pod completion"); err != nil {
		return ctrl.Result{}, err
	}

	duration := search.Status.CompletionTime.Sub(search.Status.StartTime.Time)
	searchDurationSeconds.With(metricLabels).Observe(duration.Seconds())

	return ctrl.Result{}, nil
}

func generateBaseSearchEnvironment(search *v1beta1.Search, crawlerName string) []corev1.EnvVar {
	var schedulerInterval = 60
	if search.Spec.Scheduler.IntervalSeconds != nil {
		schedulerInterval = int(*search.Spec.Scheduler.IntervalSeconds)
	}
	return []corev1.EnvVar{
		{
			Name:      v1beta1.EnvVarSearchName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		},
		{
			Name:  v1beta1.EnvVarCrawlerName,
			Value: crawlerName,
		},
		{
			Name:      v1beta1.EnvVarNamespaceName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
		},
		{
			Name:  v1beta1.EnvVarPipelineTemplatePath,
			Value: pipelineTemplatePath,
		},
		{
			Name:  v1beta1.EnvVarPipelineFIFO,
			Value: pipelineFIFOPath,
		},
		{
			Name:  v1beta1.EnvVarSearchFIFO,
			Value: searchFIFOPath,
		},
		{
			Name:  v1beta1.EnvVarPipelineSchedulerIntervalSeconds,
			Value: strconv.Itoa(schedulerInterval),
		},
		{
			Name:  v1beta1.EnvVarSidecarSchedulerCompletePath,
			Value: sidecarCompletePath,
		},
	}
}

const (
	// sidecarSchedulerContainerName is the name of the container
	// that runs the sidecar in scheduler mode
	sidecarSchedulerContainerName = "scheduler"
	// sidecarKeepaliveContainerName is the name of the container
	// that keeps the pod alive until the scheduler is complete
	sidecarKeepaliveContainerName = "scheduler-keepalive"
	// sidecarInitContainerName is the name of the container
	// that awaits the sidecar to be running
	sidecarInitContainerName = "scheulder-init"
)

func (r *SearchReconciler) generateSchedulerSidecarContainer(_ *v1beta1.Search) corev1.Container {
	var sidecarEnvVars []corev1.EnvVar

	return corev1.Container{
		Name:            sidecarExtractorPodName,
		Image:           r.SidecarImage,
		ImagePullPolicy: r.SidecarPullPolicy,
		Args:            []string{"scheduler"},
		Env:             sidecarEnvVars,
		RestartPolicy:   ptr.To(corev1.ContainerRestartPolicyAlways),
	}
}

func (r *SearchReconciler) generateKeepaliveSidecarContainer(_ *v1beta1.Search) corev1.Container {

	return corev1.Container{
		Name:            sidecarKeepaliveContainerName,
		Image:           r.SidecarImage,
		ImagePullPolicy: r.SidecarPullPolicy,
		Args:            []string{"scheduler-keepalive"},
	}
}

func (r *SearchReconciler) generateInitSidecarContainer(_ *v1beta1.Search) corev1.Container {

	return corev1.Container{
		Name:            sidecarInitContainerName,
		Image:           r.SidecarImage,
		ImagePullPolicy: r.SidecarPullPolicy,
		Args:            []string{"scheduler-init"},
	}
}

func (r *SearchReconciler) handlePostCompletion(ctx context.Context, search *v1beta1.Search) (ctrl.Result, error) {
	l := logf.FromContext(ctx).WithValues("search", search.GetName())
	if search.Spec.TTLSecondsAfterFinished != nil {
		// check if we need to delete the search
		finishTime := search.Status.CompletionTime.Time
		ttl := time.Duration(*search.Spec.TTLSecondsAfterFinished) * time.Second
		deleteTime := finishTime.Add(ttl)
		if time.Now().After(deleteTime) {
			l.Info("search has exceeded its TTL, deleting",
				"completionTime", search.Status.CompletionTime, "ttlSecondsAfterFinished", *search.Spec.TTLSecondsAfterFinished)
			if err := r.Delete(ctx, search); err != nil {
				l.Error(err, "error deleting search after TTL exceeded")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else {
			l.Info("search has completed, checking TTL before next reconciliation",
				"completionTime", search.Status.CompletionTime, "ttlSecondsAfterFinished", *search.Spec.TTLSecondsAfterFinished)
			return ctrl.Result{RequeueAfter: time.Until(deleteTime)}, nil
		}
	}
	l.Info("search has completed, skipping reconciliation", "completionTime", search.Status.CompletionTime)
	return ctrl.Result{}, nil
}
