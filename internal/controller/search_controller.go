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
	"maps"
	"strconv"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/containers"
	"github.com/crashappsec/ocular/internal/resources"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The Search reconciler is responsible for creating and managing the search pod,
// role binding, and service account.
// It ensures that the search pod is created and tied to the service account
// with the necessary permissions. Users can supply a service account in which case
// a role binding will reference it but the search controller will not update.
// Breakdown of the reconciliation steps:
// 1. Fetch the search instance
// 2. Fetch referenced resources (crawler)
// 4. Fetch or create serivceaccount (exit if created)
// 5. Fetch or create role binding (exit if created)
// 6. Fetch or create pod (exit if created)
// 7. Continually Update the search status accordingly based on the state of the pod
// 9. Once completed, await TTL if set
// For more details, check Reconcile and its Result here:
// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
func (r *SearchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("reconciling search", "name", req.Name, "namespace", req.Namespace)

	// Fetch the Pipeline instance to be reconciled
	search := &v1beta1.Search{}
	err := r.Get(ctx, req.NamespacedName, search)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	l = l.WithValues("search", search.Name, "namespace", search.Namespace)

	// If the pipeline has a completion time, handle post-completion logic
	if search.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, search)
	}

	crawler, err := resources.CrawlerInvocationFromReference(ctx, r.Client, req.Namespace, search.Spec.CrawlerRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: search.Spec.ServiceAccountName, Namespace: search.GetNamespace()}}
	l = l.WithValues("serviceAccount", serviceAccount.Name)
	serviceAccountOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, serviceAccount, func() error {
		return r.populateServiceAccount(search, serviceAccount)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to generate service account: %w", err)
	}

	if serviceAccountOp == controllerutil.OperationResultCreated ||
		serviceAccountOp == controllerutil.OperationResultUpdated {
		l.Info("serivce account was modified")
		return ctrl.Result{}, nil
	}

	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: search.GetName() + searchResourceSuffix, Namespace: search.Namespace}}
	l = l.WithValues("roleBinding", roleBinding.Name)
	roleBindingOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, roleBinding, func() error {
		return r.populateRoleBinding(search, serviceAccount, roleBinding)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to generate role binding: %w", err)
	}
	if roleBindingOp == controllerutil.OperationResultCreated ||
		roleBindingOp == controllerutil.OperationResultUpdated {
		l.Info("serivce account was modified")
		return ctrl.Result{}, nil
	}

	searchPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: search.Name + searchResourceSuffix, Namespace: search.Namespace}}
	l = l.WithValues("pod", searchPod.Name)
	searchPodOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, searchPod, func() error {
		return r.populateSearchPod(search, searchPod, crawler)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to generate search pod: %w", err)
	}
	if searchPodOp == controllerutil.OperationResultCreated ||
		searchPodOp == controllerutil.OperationResultUpdated {
		l.Info("serivce account was modified")
		return ctrl.Result{}, nil
	}

	// Update status to reflect pods have been created
	if search.Status.StartTime == nil {
		l.Info("marking search as started")
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
		return ctrl.Result{}, updateStatus(ctx, r.Client, search)
	}

	// Check for completion of pods and update status accordingly
	return r.handleCompletion(ctx, search, searchPod)
}

func (r *SearchReconciler) populateServiceAccount(search *v1beta1.Search, sa *corev1.ServiceAccount) error {
	// we only set the controller reference if we are creating the role, since users can
	// bring their own roles, owned by other controllers.
	if sa.CreationTimestamp.IsZero() {
		if sa.Labels == nil {
			sa.Labels = make(map[string]string)
		}
		sa.Labels[v1beta1.SearchLabelKey] = search.GetName()
		sa.Labels[v1beta1.TypeLabelKey] = v1beta1.ServiceAccountTypeSearch
		return ctrl.SetControllerReference(search, sa, r.Scheme)
	}
	return nil
}

func (r *SearchReconciler) populateRoleBinding(search *v1beta1.Search, sa *corev1.ServiceAccount, rb *rbacv1.RoleBinding) error {
	if rb.Labels == nil {
		rb.Labels = make(map[string]string)
	}
	rb.Labels[v1beta1.SearchLabelKey] = search.GetName()
	rb.Labels[v1beta1.TypeLabelKey] = v1beta1.RoleBindingTypeSearch

	rb.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     r.SearchClusterRole,
	}
	rb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      sa.GetName(),
			Namespace: sa.GetNamespace(),
		},
	}
	return ctrl.SetControllerReference(search, rb, r.Scheme)

}

func (r *SearchReconciler) populateSearchPod(search *v1beta1.Search, pod *corev1.Pod, crawler resources.Invocation[v1beta1.CrawlerSpec]) error {
	// Once a pod is created not much can change
	// and to avoid conflicts with defaulted values
	// we skip if the pod is created
	if pod.CreationTimestamp.IsZero() {

		envVars := generateBaseSearchEnvironment(search)
		containerOpts := generateBaseContainerOptions(envVars)

		var crawlerContainer corev1.Container
		crawler.Spec.Container.DeepCopyInto(&crawlerContainer)

		crawlerContainer.Env = append(crawlerContainer.Env,
			containers.ParseParameterEnvVars(crawler.Spec.Parameters, crawler.Parameters)...)
		templateVolume := corev1.Volume{
			Name: "pipeline-template",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{{
						Path:     pipelineTemplateFile,
						FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations['" + v1beta1.PipelineTemplateAnnotation + "']"}}},
				},
			},
		}
		socketVolume := corev1.Volume{
			Name:         "pipeline-socket",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}},
		}

		pipelineTemplateJSON, err := json.Marshal(search.Spec.Scheduler.PipelineTemplate)
		if err != nil {
			return fmt.Errorf("unable to marshal pipeline template: %w", err)
		}

		containerOpts = append(containerOpts,
			containers.WithAdditionalEnvVars(
				corev1.EnvVar{
					Name: v1beta1.EnvVarSchedulerSearchTTL,
					ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['" + v1beta1.TTLSecondsAnnotation + "']",
					}}},
				corev1.EnvVar{
					Name: v1beta1.EnvVarSchedulerServiceAccount,
					ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['" + v1beta1.ServiceAccountNameAnnotation + "']",
					}}},
			),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      templateVolume.Name,
				MountPath: pipelineTemplateDir,
			}, corev1.VolumeMount{
				Name:      socketVolume.Name,
				MountPath: ocularFIFODir,
			}))

		schedulerSidecarContainer := r.generateSchedulerSidecarContainer(search)
		keepaliveSidecarContainer := r.generateKeepaliveSidecarContainer(search)
		initSidecarContainer := r.generateInitSidecarContainer(search)

		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		maps.Copy(pod.Labels, resources.PropagateMetadata(search.Labels, crawler.Metadata.Labels))
		pod.Labels[v1beta1.SearchLabelKey] = search.GetName()
		pod.Labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeSearch
		pod.Labels[v1beta1.CrawlerLabelKey] = search.Spec.CrawlerRef.Name

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		maps.Copy(pod.Annotations, resources.PropagateMetadata(search.Annotations, crawler.Metadata.Annotations))
		pod.Annotations[v1beta1.PipelineTemplateAnnotation] = string(pipelineTemplateJSON)
		pod.Annotations[v1beta1.ServiceAccountNameAnnotation] = search.Spec.ServiceAccountName
		if search.Spec.TTLSecondsAfterFinished != nil {
			pod.Annotations[v1beta1.TTLSecondsAnnotation] = strconv.Itoa(int(*search.Spec.TTLSecondsAfterFinished))
		} else {
			pod.Annotations[v1beta1.TTLSecondsAnnotation] = ""
		}

		pod.Spec.ServiceAccountName = search.Spec.ServiceAccountName
		pod.Spec.RuntimeClassName = search.Spec.RuntimeClassName
		pod.Spec.RestartPolicy = corev1.RestartPolicyNever
		pod.Spec.InitContainers = containers.ApplyOptions([]corev1.Container{
			// 1. we start the side car scheduler
			schedulerSidecarContainer,
			// then wait till the scheduler is ready
			initSidecarContainer,
			// then let the user container run
			crawlerContainer,
		}, containerOpts...)
		pod.Spec.Containers = containers.ApplyOptions([]corev1.Container{keepaliveSidecarContainer}, containerOpts...)
		pod.Spec.Volumes = append(crawler.Spec.Volumes, templateVolume, socketVolume)
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			// TODO(fix)
			RunAsUser: new(int64(0)),
		}
		pod.Spec.ImagePullSecrets = crawler.Spec.ImagePullSecrets
	}
	return ctrl.SetControllerReference(search, pod, r.Scheme)
}

func (r *SearchReconciler) handleCompletion(ctx context.Context, search *v1beta1.Search, pod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	childPipelines, childSearches, err := r.retrieveRunningScheduledResources(ctx, search)
	if err != nil {
		return ctrl.Result{}, err
	} else if len(childPipelines) > 0 || len(childSearches) > 0 {
		l.Info("child resources not completed", "search", search.Name, "pipelines", childPipelines, "searches", childSearches)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	metricLabels := prometheus.Labels{"namespace": search.Namespace, "crawler": search.Spec.CrawlerRef.Name}
	switch pod.Status.Phase {
	case corev1.PodFailed:
		l.Info("search pod has failed", "name", search.GetName(), "pod", pod.GetName())
		if search.Status.CompletionTime == nil {
			t := metav1.NewTime(time.Now())
			search.Status.CompletionTime = new(t)
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
			search.Status.CompletionTime = new(t)
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

func (r *SearchReconciler) retrieveRunningScheduledResources(ctx context.Context, search *v1beta1.Search) ([]v1beta1.Pipeline, []v1beta1.Search, error) {
	var (
		err error

		pipelineList v1beta1.PipelineList
		searchList   v1beta1.SearchList
	)
	err = r.List(ctx, &pipelineList, &client.ListOptions{
		Namespace: search.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			v1beta1.ScheduledByLabelKey: search.Name,
		}),
	})
	if err != nil {
		return nil, nil, err
	}

	err = r.List(ctx, &searchList, &client.ListOptions{
		Namespace: search.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			v1beta1.ScheduledByLabelKey: search.Name,
		}),
	})
	if err != nil {
		return nil, nil, err
	}

	pipelines := make([]v1beta1.Pipeline, 0, len(pipelineList.Items))
	for _, p := range pipelineList.Items {
		if p.Status.CompletionTime == nil {
			pipelines = append(pipelines, p)
		}
	}

	searches := make([]v1beta1.Search, 0, len(searchList.Items))
	for _, s := range searchList.Items {
		if s.Status.CompletionTime == nil {
			searches = append(searches, s)
		}
	}
	return pipelines, searches, nil

}

func generateBaseSearchEnvironment(search *v1beta1.Search) []corev1.EnvVar {
	var schedulerInterval = 60
	if search.Spec.Scheduler.IntervalSeconds != nil {
		schedulerInterval = int(*search.Spec.Scheduler.IntervalSeconds)
	}
	return []corev1.EnvVar{
		{
			Name:  v1beta1.EnvVarSearchName,
			Value: search.Name,
		},
		{
			Name:      v1beta1.EnvVarPodName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		},
		{
			Name:  v1beta1.EnvVarCrawlerName,
			Value: search.Spec.CrawlerRef.Name,
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
			Name:  v1beta1.EnvVarSchedulerCompletePath,
			Value: sidecarCompletePath,
		},
		{
			Name:  v1beta1.EnvVarSchedulerParentUID,
			Value: string(search.UID),
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
		Name:            sidecarSchedulerContainerName,
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
