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
	"fmt"
	"maps"
	"path"
	"slices"
	"time"

	"github.com/crashappsec/ocular/internal/containers"
	"github.com/crashappsec/ocular/internal/resources"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/crashappsec/ocular/api/v1beta1"
)

var (
	pipelinesCompleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipelines_completed_total",
			Help: "Number of ocular pipelines created",
		},
		[]string{"profile", "downloader", "namespace", "phase"},
	)
	pipelinesRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipelines_active",
			Help: "Number of ocular pipelines running currently",
		},
		[]string{"profile", "downloader", "namespace"},
	)
	scanPodsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_scan_pods_total",
			Help: "Number of scan pods ocular has created",
		},
		[]string{"profile", "downloader", "namespace"},
	)
	uploadPodsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_upload_pods_total",
			Help: "Number of scan pods ocular has created",
		},
		[]string{"profile", "downloader", "namespace"},
	)
	pipelineDurationSeconds = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "pipeline_duration_seconds",
			Help: "Duration in seconds of pipeline execution, by terminal phase.",
		},
		[]string{"profile", "downloader", "namespace", "phase"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		pipelinesCompleted,
		pipelinesRunning,
		scanPodsCreated, uploadPodsCreated,
		pipelineDurationSeconds,
	)
}

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	SidecarImage      string
	SidecarPullPolicy corev1.PullPolicy
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Pipeline{}).
		Named("pipeline").
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles;downloaders;uploaders,verbs=get;list;watch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=clusterprofiles;clusterdownloaders;clusteruploaders,verbs=get;list;watch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services;pods,verbs=watch;create;get;list;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The pipeline reconciler is responsible for creating and managing the scan pod, upload pod
// (if applicable) and upload service (if applicable) for a given pipeline.
// It ensures that the pods are created and updated as necessary,
// and that the status of the pipeline is updated accordingly.
// Breakdown of the reconciliation steps:
// 1. Fetch the pipeline instance
// 2. Fetch referenced resources (profile, downloader, uploaders)
// 3. Update status to indicate if scan only (exit if updated)
// 4. Fetch or create upload pod if applicable (exit if created)
// 5. Fetch or create upload service if applicable (exit if created)
// 6. Await upload pod running, and create scan pod if applicable (exit if created)
// 8. Continually Update the pipeline status accordingly based on the state of the pods
// 9. Once completed, await TTL if set
// For more details, check Reconcile and its Result here:
// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("reconciling pipeline object", "name", req.Name, "namespace", req.Namespace, "req", req)

	// Fetch the Pipeline instance to be reconciled
	pipeline := &v1beta1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	l = l.WithValues("pipeline", pipeline.Name, "namespace", pipeline.Namespace)
	ctx = logf.IntoContext(ctx, l)

	wasDeleted := !pipeline.DeletionTimestamp.IsZero()
	if wasDeleted && controllerutil.ContainsFinalizer(pipeline, metricsFinalizer) {
		patch := client.MergeFrom(pipeline.DeepCopy())
		controllerutil.RemoveFinalizer(pipeline, metricsFinalizer)
		if err := patchResource(ctx, r.Client, pipeline, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove metrics finalizer upon deletion: %w", err)
		}
		pipelinesRunning.With(metricLabelsForPipeline(pipeline)).Dec()
		return ctrl.Result{}, nil
	}

	if pipeline.Spec.TTLSecondsMaxLifetime != nil {
		ttlMaxSeconds := float64(*pipeline.Spec.TTLSecondsMaxLifetime)
		if time.Since(pipeline.GetCreationTimestamp().Time).Seconds() > ttlMaxSeconds {
			l.Info("pipeline has exceeded maximum allowed runtime, cleaning up", "max-ttl", ttlMaxSeconds)
			err := r.Delete(ctx, pipeline)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	// If the pipeline has a completion time, handle post-completion logic
	if !pipeline.Status.CompletionTime.IsZero() {
		return r.handlePostCompletion(ctx, pipeline)
	}

	profile, err := resources.ProfileInvocationFromReference(ctx, r.Client, pipeline.GetNamespace(), pipeline.Spec.ProfileRef)
	if err != nil {
		return ctrl.Result{}, err
	}
	l = l.WithValues("profile", pipeline.Spec.ProfileRef)

	downloader, err := resources.DownloaderInvocationFromReference(ctx, r.Client, pipeline.GetNamespace(), pipeline.Spec.DownloaderRef)
	if err != nil {
		return ctrl.Result{}, err
	}
	l = l.WithValues("downloader", pipeline.Spec.DownloaderRef)

	uploaders, err := uploaderInvocationsFromProfile(ctx, r.Client, pipeline.Namespace, profile.Spec.UploaderRefs)
	if err != nil {
		return ctrl.Result{}, err
	}

	// In the case where no artifacts or uploaders are defined and the pipeline hasn't started
	// set the status to scan pod only
	scanPodOnly := len(profile.Spec.UploaderRefs) == 0
	if pipeline.Status.StartTime == nil && scanPodOnly != pipeline.Status.ScanPodOnly {
		l.Info("setting pipeline scan only status", "scanPodOnly", scanPodOnly)
		patch := client.MergeFrom(pipeline.DeepCopy())
		pipeline.Status.ScanPodOnly = scanPodOnly
		if scanPodOnly {
			pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageSkipped
		}
		return ctrl.Result{}, patchStatus(logf.IntoContext(ctx, l), r.Client, pipeline, patch)
	}
	l = l.WithValues("scanPodOnly", pipeline.Status.ScanPodOnly)

	envVars := generateBasePipelineEnvironment(pipeline)
	containerOpts := generateBaseContainerOptions(envVars)
	// aritfactArgs are the arguments passed to the sidecar
	// & uploaders to specify which artifacts to extract
	artifactArgs := generateArtifactArguments(downloader.Spec.MetadataFiles, profile.Spec.Artifacts)

	// generate upload components (if applicable)
	uploadPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pipeline.GetName() + uploadSuffix, Namespace: pipeline.GetNamespace()}}
	uploadService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: pipeline.GetName() + uploadSuffix, Namespace: pipeline.GetNamespace()}}
	if !pipeline.Status.ScanPodOnly {
		l.Info("ensuring upload resources are created")
		uploadPodOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, uploadPod, func() error {
			uploaderContainerOpts := append(containerOpts,
				containers.WithAdditionalArgs(artifactArgs...),
				containers.WithWorkingDir(v1beta1.PipelineResultsDirectory),
				containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
					Name:      pipelineResultsVolumeName,
					MountPath: v1beta1.PipelineResultsDirectory,
				}),
				containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
					Name:      pipelineMetadataVolumeName,
					MountPath: v1beta1.PipelineMetadataDirectory,
				}),
			)
			return r.populateUploadPod(uploadPod, pipeline, profile, uploaders, uploaderContainerOpts...)
		})
		if err != nil {
			return ctrl.Result{}, client.IgnoreAlreadyExists(fmt.Errorf("unable to generate new upload pod: %w", err))
		}

		l = l.WithValues("uploadPod", uploadPod.Name)

		switch uploadPodOp {
		case controllerutil.OperationResultCreated:
			uploadPodsCreated.With(metricLabelsForPipeline(pipeline)).Inc()
			fallthrough
		case controllerutil.OperationResultUpdated:
			l.Info("upload pod was created or modified", "op", uploadPodOp)
			return ctrl.Result{}, nil
		}

		uploadServiceOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, uploadService, func() error {
			return r.populateUploadService(uploadService, pipeline)
		})
		if err != nil {
			return ctrl.Result{}, client.IgnoreAlreadyExists(fmt.Errorf("unable to generate new upload service: %w", err))
		}

		l = l.WithValues("uploadService", uploadService.Name)
		if uploadServiceOp == controllerutil.OperationResultCreated ||
			uploadServiceOp == controllerutil.OperationResultUpdated {
			l.Info("upload service was created or modified", "op", uploadServiceOp)
			return ctrl.Result{}, nil
		}

		if pipeline.Status.StartTime.IsZero() && !uploadPodStarted(uploadPod) {
			l.Info("upload pod and service created, awaiting upload pod ready")
			return ctrl.Result{RequeueAfter: time.Second * 2}, nil
		}

	}

	scanPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pipeline.GetName() + scanSuffix, Namespace: pipeline.GetNamespace()}}
	scanPodOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, scanPod, func() error {
		sidecarContainer := r.createSidecarExtractorContainer(pipeline, uploadService, artifactArgs)
		scanContainerOpts := append(containerOpts,
			containers.WithWorkingDir(v1beta1.PipelineTargetDirectory),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineTargetVolumeName,
				MountPath: v1beta1.PipelineTargetDirectory,
			}),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineMetadataVolumeName,
				MountPath: v1beta1.PipelineMetadataDirectory,
			}),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineResultsVolumeName,
				MountPath: v1beta1.PipelineResultsDirectory,
			}))
		return r.populateScanPod(scanPod, pipeline, profile, downloader, sidecarContainer, scanContainerOpts...)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to generate new scan pod: %w", err)
	}

	l = l.WithValues("scanPod", scanPod.Name)

	switch scanPodOp {
	case controllerutil.OperationResultCreated:
		scanPodsCreated.With(metricLabelsForPipeline(pipeline)).Inc()
		fallthrough
	case controllerutil.OperationResultUpdated:
		return ctrl.Result{}, nil
	}

	if !pipeline.Status.StartTime.IsZero() {
		if !controllerutil.ContainsFinalizer(pipeline, metricsFinalizer) {
			patch := client.MergeFrom(pipeline.DeepCopy())
			controllerutil.AddFinalizer(pipeline, metricsFinalizer)
			if err := patchResource(ctx, r.Client, pipeline, patch); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add metrics finalizer: %w", err)
			}
			pipelinesRunning.With(metricLabelsForPipeline(pipeline)).Inc()
			return ctrl.Result{}, nil
		}

		// Check for completion of pods and update status accordingly
		return r.handleCompletion(logf.IntoContext(ctx, l), pipeline, scanPod, uploadPod)
	}

	// Update status to reflect pods have been created
	l.Info("marking pipeline as started")
	startTime := metav1.Now()
	patch := client.MergeFrom(pipeline.DeepCopy())
	reason, message := "ScanPodSuccessfullyCreated", fmt.Sprintf("The scan pod %s has been created.", scanPod.Name)
	pipeline.Status.Conditions = append(pipeline.Status.Conditions, metav1.Condition{
		Type:               v1beta1.PipelineScanPodCreatedConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: startTime.Rfc3339Copy(),
	})

	if !pipeline.Status.ScanPodOnly {
		reason, message = "UploadPodSuccessfullyCreated", fmt.Sprintf("The upload pod %s has been created.", uploadPod.GetName())
		pipeline.Status.Conditions = append(pipeline.Status.Conditions, metav1.Condition{
			Type:               v1beta1.PipelineUploadPodCreatedConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: startTime,
		})
		pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageNotStarted
	}
	pipeline.Status.StartTime = &startTime
	pipeline.Status.Phase = v1beta1.PipelineDownloading
	pipeline.Status.StageStatuses.DownloadStatus = v1beta1.PipelineStageInProgress
	pipeline.Status.StageStatuses.ScanStatus = v1beta1.PipelineStageNotStarted
	err = patchStatus(logf.IntoContext(ctx, l), r.Client, pipeline, patch)
	return ctrl.Result{}, err

}

const (
	// sidecarExtractorContainerName is the name of the sidecar container in the scan pod
	// that handles extracting artifacts and uploading them to the upload pod
	sidecarExtractorContainerName = "extract-artifacts"

	// sidecarReceiverContainerName is the name of the receiver
	// sidecar container in the upload pod
	sidecarReceiverContainerName = "receive-artifacts"
)

func (r *PipelineReconciler) createSidecarExtractorContainer(pipeline *v1beta1.Pipeline, uploadService *corev1.Service, artifactsArgs []string) corev1.Container {
	var (
		sidecarEnvVars []corev1.EnvVar
		sidecarCommand = "ignore"
	)

	if !pipeline.Status.ScanPodOnly {
		sidecarCommand = "extract"
		if uploadService != nil {
			sidecarEnvVars = append(sidecarEnvVars, corev1.EnvVar{
				Name:  v1beta1.EnvVarExtractorHost,
				Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", uploadService.Name, uploadService.Namespace, extractorPort),
			})
		}
	}

	return corev1.Container{
		Name:            sidecarExtractorContainerName,
		Image:           r.SidecarImage,
		ImagePullPolicy: r.SidecarPullPolicy,
		Args:            append([]string{sidecarCommand}, artifactsArgs...),
		Env:             sidecarEnvVars,
		RestartPolicy:   new(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: new(true),
		},
	}
}

func (r *PipelineReconciler) handleCompletion(ctx context.Context, pipeline *v1beta1.Pipeline, scanPod, uploadPod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("checking for scan & upload pod completion")

	t := metav1.NewTime(time.Now())
	patch := client.MergeFrom(pipeline.DeepCopy())

	switch scanPod.Status.Phase {
	case corev1.PodSucceeded:
		// scan pod completed successfully
		pipeline.Status.StageStatuses.DownloadStatus = v1beta1.PipelineStageCompleted
		pipeline.Status.StageStatuses.ScanStatus = v1beta1.PipelineStageCompleted
		if pipeline.Status.ScanPodOnly {
			pipeline.Status.Phase = v1beta1.PipelineSucceeded
			pipeline.Status.CompletionTime = new(t)
			pipeline.Status.Conditions = append(pipeline.Status.Conditions,
				metav1.Condition{
					Type:               v1beta1.PipelineCompletedSuccessfullyConditionType,
					Status:             metav1.ConditionTrue,
					Reason:             "ScanCompletedSuccessfully",
					Message:            "The scan pod has completed successfully.",
					LastTransitionTime: t,
				})
		} else {
			// if we have an upload pod, wait for it to complete
			switch uploadPod.Status.Phase {
			case corev1.PodSucceeded:
				pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageCompleted
				pipeline.Status.Phase = v1beta1.PipelineSucceeded
				pipeline.Status.CompletionTime = new(t)
				pipeline.Status.Conditions = append(pipeline.Status.Conditions,
					metav1.Condition{
						Type:               v1beta1.PipelineCompletedSuccessfullyConditionType,
						Status:             metav1.ConditionTrue,
						Reason:             "ScanAndUploadPodComplete",
						Message:            "The scan and upload pods have completed successfully.",
						LastTransitionTime: t,
					})
			case corev1.PodFailed:
				pipeline.Status.CompletionTime = new(t)
				pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageFailed
				pipeline.Status.Conditions = append(pipeline.Status.Conditions,
					metav1.Condition{
						Type:               v1beta1.PipelineCompletedSuccessfullyConditionType,
						Status:             metav1.ConditionFalse,
						Reason:             "UploadPodTerminatedWithFailures",
						Message:            "The upload pod has failed.",
						LastTransitionTime: t,
					})
				pipeline.Status.Phase = v1beta1.PipelineFailed
			case corev1.PodRunning, corev1.PodPending:
				// upload pod still running or pending
				uploadStatus := determineUploadPodStageStatuses(uploadPod)
				pipeline.Status.StageStatuses.UploadStatus = uploadStatus
				pipeline.Status.Phase = v1beta1.PipelineUploading
				return ctrl.Result{}, patchStatus(ctx, r.Client, pipeline, patch)
			default:
				// upload pod in unknown state, requeue for further investigation
				l.Error(fmt.Errorf("upload pod in unknown state"), "upload pod is in an unknown state", "phase", uploadPod.Status.Phase, "name", pipeline.GetName())
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
	case corev1.PodFailed:
		downloaderStatus, scanStatus := determineScanPodStageStatuses(scanPod)
		pipeline.Status.StageStatuses.DownloadStatus = downloaderStatus
		pipeline.Status.StageStatuses.ScanStatus = scanStatus
		pipeline.Status.Phase = v1beta1.PipelineFailed
		pipeline.Status.CompletionTime = new(t)
		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               v1beta1.CompletedSuccessfullyConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "ScanPodTerminatedWithFailures",
				Message:            "The scan and/or upload pod have failed.",
				LastTransitionTime: t,
			})
		if !pipeline.Status.ScanPodOnly && slices.Contains([]corev1.PodPhase{
			corev1.PodPending,
		}, uploadPod.Status.Phase) {
			// if we have an upload pod still running or pending, we should clean it up
			// TODO(bryce): cleanup pod
			return ctrl.Result{}, r.failPod(ctx, uploadPod)
		}
	case corev1.PodRunning:
		downloaderStatus, scanStatus := determineScanPodStageStatuses(scanPod)
		if downloaderStatus == v1beta1.PipelineStageInProgress {
			pipeline.Status.Phase = v1beta1.PipelineDownloading
		} else if scanStatus == v1beta1.PipelineStageInProgress {
			pipeline.Status.Phase = v1beta1.PipelineScanning
		}
		return ctrl.Result{}, patchStatus(ctx, r.Client, pipeline, patch)
	case corev1.PodPending:
		// scan pod still running or pending
		return ctrl.Result{}, nil
	default:
		// scan pod in unknown state, requeue for further investigation
		l.Error(fmt.Errorf("scan pod in unknown state"), "scan pod is in an unknown state", "phase", scanPod.Status.Phase, "name", pipeline.GetName())
		pipeline.Status.Phase = v1beta1.PipelineStateUnknown
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	l = l.WithValues("phase", pipeline.Status.Phase)
	err := patchStatus(logf.IntoContext(ctx, l), r.Client, pipeline, patch)
	return ctrl.Result{}, err
}

func uploadPodStarted(p *corev1.Pod) bool {
	// check if uploader is running & can accept artifacts
	// or if its terminated (i.e. already ran)
	for _, status := range p.Status.InitContainerStatuses {
		if status.Name == sidecarReceiverContainerName {
			return status.State.Running != nil || status.State.Terminated != nil
		}
	}
	return false
}

func (r *PipelineReconciler) failPod(ctx context.Context, pod *corev1.Pod) error {
	l := logf.FromContext(ctx)
	l.Info("failing pod", "name", pod.GetName())
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		return nil
	}
	pod.Spec.ActiveDeadlineSeconds = ptr.To[int64](1)
	return r.Update(ctx, pod)
}

func (r *PipelineReconciler) populateUploadService(svc *corev1.Service, pipeline *v1beta1.Pipeline) error {
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}

	svc.Labels[v1beta1.TypeLabelKey] = v1beta1.ServiceTypeUpload
	svc.Labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
	svc.Labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name
	svc.Labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name
	if svc.Spec.Selector == nil {
		svc.Spec.Selector = make(map[string]string)
	}

	svc.Spec.Selector[v1beta1.PipelineLabelKey] = pipeline.GetName()
	svc.Spec.Selector[v1beta1.TypeLabelKey] = v1beta1.PodTypeUpload

	svc.Spec.PublishNotReadyAddresses = true
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != extractorPort {
		svc.Spec.Ports = []corev1.ServicePort{
			{Port: extractorPort, TargetPort: intstr.FromInt32(extractorPort), Protocol: corev1.ProtocolTCP},
		}
	}

	return ctrl.SetControllerReference(pipeline, svc, r.Scheme)
}

func (r *PipelineReconciler) populateUploadPod(pod *corev1.Pod, pipeline *v1beta1.Pipeline, profile resources.Invocation[v1beta1.ProfileSpec], uploaders []resources.Invocation[v1beta1.UploaderSpec], containerOpts ...containers.Option) error {

	if pod.CreationTimestamp.IsZero() {
		// only edit pod spec if not created yet
		// since once created, spec cant really be modified
		uploaderContainers := make([]corev1.Container, 0, len(uploaders))
		uploaderLabels := make(map[string]string)
		uploaderAnnotations := make(map[string]string)

		parentParams := resources.ParseParameters(profile.Spec.Parameters, profile.Parameters, nil)

		pod.Spec.Volumes = profile.Spec.Volumes
		pod.Spec.ImagePullSecrets = make([]corev1.LocalObjectReference, 0)
		for _, invocation := range uploaders {
			baseContainer := invocation.Spec.Container
			baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
				Name:  v1beta1.EnvVarUploaderName,
				Value: invocation.Metadata.Name,
			})

			baseContainer.Env = append(baseContainer.Env,
				containers.ParseParameterEnvVars(invocation.Spec.Parameters, invocation.Parameters, parentParams)...)

			maps.Copy(uploaderLabels, invocation.Metadata.GetLabels())
			maps.Copy(uploaderAnnotations, invocation.Metadata.GetAnnotations())

			pod.Spec.Volumes = append(pod.Spec.Volumes, invocation.Spec.Volumes...)

			uploaderContainers = append(uploaderContainers, baseContainer)
			pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, invocation.Spec.ImagePullSecrets...)
		}

		pod.Spec.ImagePullSecrets = slices.CompactFunc(pod.Spec.ImagePullSecrets, func(s1, s2 corev1.LocalObjectReference) bool {
			return s1.Name == s2.Name
		})

		sidecarContainer := corev1.Container{
			Name:  sidecarReceiverContainerName,
			Image: r.SidecarImage,

			Args: []string{"receive"},
			Env: []corev1.EnvVar{
				{Name: v1beta1.EnvVarExtractorPort, Value: fmt.Sprintf("%d", extractorPort)},
			},
			// RestartPolicy: ptr.To(corev1.ContainerRestartPolicyNever),
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: new(true),
			},
		}
		pod.Spec.ServiceAccountName = pipeline.Spec.UploadServiceAccountName
		pod.Spec.RuntimeClassName = pipeline.Spec.RuntimeClassName
		pod.Spec.RestartPolicy = corev1.RestartPolicyNever
		pod.Spec.Containers = containers.ApplyOptions(uploaderContainers, containerOpts...)
		pod.Spec.InitContainers = containers.ApplyOptions([]corev1.Container{
			// Add the extractor as an init container running in receive mode
			sidecarContainer,
		}, containerOpts...)
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			// add shared volume for target and results
			corev1.Volume{Name: pipelineTargetVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, corev1.Volume{Name: pipelineResultsVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, corev1.Volume{Name: pipelineMetadataVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		)

		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		maps.Copy(pod.Labels, resources.PropagateMetadata(pipeline.Labels, profile.Metadata.Labels, uploaderLabels))
		pod.Labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeUpload
		pod.Labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
		pod.Labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name
		pod.Labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		maps.Copy(pod.Annotations, resources.PropagateMetadata(pipeline.Annotations, profile.Metadata.Annotations, uploaderAnnotations))

	}

	return ctrl.SetControllerReference(pipeline, pod, r.Scheme)
}

func (r *PipelineReconciler) populateScanPod(pod *corev1.Pod, pipeline *v1beta1.Pipeline, profile resources.Invocation[v1beta1.ProfileSpec], downloader resources.Invocation[v1beta1.DownloaderSpec],
	extractorContainer corev1.Container, containerOpts ...containers.Option) error {
	// only edit pod spec if not created yet
	// since once created, spec cant really be modified
	if pod.CreationTimestamp.IsZero() {

		downloaderContainer := downloader.Spec.Container
		downloaderContainer.Env = append(downloaderContainer.Env, containers.ParseParameterEnvVars(downloader.Spec.Parameters, pipeline.Spec.DownloaderRef.Parameters, nil)...)

		scannerContainers := containers.FilterConditionalContainers(profile.Spec.Containers, profile.Spec.Parameters, pipeline.Spec.ProfileRef.Parameters)

		pod.Spec.ServiceAccountName = pipeline.Spec.ScanServiceAccountName
		pod.Spec.RuntimeClassName = pipeline.Spec.RuntimeClassName
		pod.Spec.RestartPolicy = corev1.RestartPolicyNever
		pod.Spec.Containers = containers.ApplyOptions(scannerContainers, append(containerOpts,
			containers.WithParameters(profile.Spec.Parameters, pipeline.Spec.ProfileRef.Parameters, nil))...,
		)

		extractorContainer.VolumeMounts = append(extractorContainer.VolumeMounts,
			corev1.VolumeMount{
				Name:      pipelineUploadTokenVolumeName,
				MountPath: v1beta1.UploadTokenDir,
				ReadOnly:  true,
			})

		pod.Spec.InitContainers = containers.ApplyOptions([]corev1.Container{
			// Add the downloader as an init container
			downloaderContainer,
			// Add the extractor as a sidecar container running in extract mode
			extractorContainer,
		}, containerOpts...)
		volumes := append(profile.Spec.Volumes, downloader.Spec.Volumes...)
		pod.Spec.Volumes = append(volumes,
			corev1.Volume{Name: pipelineTargetVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, corev1.Volume{Name: pipelineResultsVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, corev1.Volume{Name: pipelineMetadataVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{Name: pipelineUploadTokenVolumeName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Audience:          pipeline.GetName() + uploadSuffix,
									ExpirationSeconds: new(int64(60 * 60)),
									Path:              v1beta1.UploadTokenFile,
								},
							},
						},
					},
				},
			},
		)

		pod.Spec.ImagePullSecrets = slices.CompactFunc(
			append(profile.Spec.ImagePullSecrets, downloader.Spec.ImagePullSecrets...),
			func(s1, s2 corev1.LocalObjectReference) bool { return s1.Name == s2.Name },
		)

		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		maps.Copy(pod.Labels, resources.PropagateMetadata(downloader.Metadata.GetLabels(), profile.Metadata.GetLabels()))
		pod.Labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeScan
		pod.Labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
		pod.Labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name
		pod.Labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		maps.Copy(pod.Annotations, resources.PropagateMetadata(downloader.Metadata.GetAnnotations(), profile.Metadata.GetAnnotations()))

	}

	return ctrl.SetControllerReference(pipeline, pod, r.Scheme)
}

func generateArtifactArguments(metadataFiles []string, artifacts []string) []string {
	args := []string{"--"}
	for _, artifact := range artifacts {
		artifactPath := path.Clean(artifact)
		if path.IsAbs(artifactPath) {
			args = append(args, artifactPath)
		} else {
			args = append(args, path.Join(v1beta1.PipelineResultsDirectory, artifactPath))
		}
	}
	for _, artifact := range metadataFiles {
		artifactPath := path.Clean(artifact)
		if path.IsAbs(artifactPath) {
			args = append(args, artifactPath)
		} else {
			args = append(args, path.Join(v1beta1.PipelineMetadataDirectory, artifactPath))
		}
	}
	return args
}

func generateBasePipelineEnvironment(pipeline *v1beta1.Pipeline) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  v1beta1.EnvVarTargetIdentifier,
			Value: pipeline.Spec.Target.Identifier,
		},
		{
			Name:  v1beta1.EnvVarTargetVersion,
			Value: pipeline.Spec.Target.Version,
		},
		{
			Name:  v1beta1.EnvVarDownloaderName,
			Value: pipeline.Spec.DownloaderRef.Name,
		},
		{
			Name:  v1beta1.EnvVarProfileName,
			Value: pipeline.Spec.ProfileRef.Name,
		},
		{
			Name:  v1beta1.EnvVarPipelineName,
			Value: pipeline.Name,
		},
		{
			Name:      v1beta1.EnvVarPodName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		},
		{
			Name:  v1beta1.EnvVarTargetDir,
			Value: v1beta1.PipelineTargetDirectory,
		},
		{
			Name:  v1beta1.EnvVarResultsDir,
			Value: v1beta1.PipelineResultsDirectory,
		},
		{
			Name:  v1beta1.EnvVarMetadataDir,
			Value: v1beta1.PipelineMetadataDirectory,
		},
		{
			Name:      v1beta1.EnvVarNamespaceName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
		},
	}

}

func (r *PipelineReconciler) handlePostCompletion(ctx context.Context, pipeline *v1beta1.Pipeline) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("handling post completion")
	if controllerutil.ContainsFinalizer(pipeline, metricsFinalizer) {
		patch := client.MergeFrom(pipeline.DeepCopy())
		controllerutil.RemoveFinalizer(pipeline, metricsFinalizer)
		if err := patchResource(ctx, r.Client, pipeline, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer at completion: %w", err)
		}
		metricLabels := metricLabelsForPipeline(pipeline)
		pipelinesRunning.With(metricLabels).Dec()
		metricLabels["phase"] = string(pipeline.Status.Phase)
		duration := pipeline.Status.CompletionTime.Sub(pipeline.Status.StartTime.Time)
		pipelinesCompleted.With(metricLabels).Add(1)
		pipelineDurationSeconds.With(metricLabels).Observe(duration.Seconds())
		l.Info("pipeline metrics updated with completion",
			"pipeline", pipeline.Name, "namespace", pipeline.Namespace,
			"profile", pipeline.Spec.ProfileRef.Name, "downloader", pipeline.Spec.DownloaderRef.Name,
			"target", pipeline.Spec.Target,
			"start-time", pipeline.Status.StartTime, "completion-time", pipeline.Status.CompletionTime)
	}
	if pipeline.Spec.TTLSecondsAfterFinished == nil {
		l.Info("pipeline has completed",
			"name", pipeline.GetName(),
			"completionTime", pipeline.Status.CompletionTime)
		return ctrl.Result{}, nil
	}

	ttl := time.Duration(*pipeline.Spec.TTLSecondsAfterFinished) * time.Second
	wait := time.Until(pipeline.Status.CompletionTime.Add(ttl))
	if wait <= 0 {
		l.Info("pipeline has exceeded its TTL, deleting",
			"name", pipeline.GetName(),
			"completionTime", pipeline.Status.CompletionTime,
			"ttl", ttl)
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, pipeline))
	}

	l.Info("pipeline has completed, checking TTL before next reconciliation",
		"name", pipeline.GetName(),
		"completion-time", pipeline.Status.CompletionTime,
		"ttl", ttl.Seconds(),
		"requeue-after", wait.String(),
	)

	return ctrl.Result{RequeueAfter: wait}, nil
}

func determineScanPodStageStatuses(scanPod *corev1.Pod) (dlStatus v1beta1.PipelineStageStatus, scanStatus v1beta1.PipelineStageStatus) {
	completed, failed := true, false
	for _, cs := range scanPod.Status.InitContainerStatuses {
		if cs.Name != sidecarExtractorContainerName {
			completed = completed && cs.State.Terminated != nil
			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				failed = true
			}
		}
	}

	if failed {
		dlStatus = v1beta1.PipelineStageFailed
	} else if completed {
		dlStatus = v1beta1.PipelineStageCompleted
	} else {
		dlStatus = v1beta1.PipelineStageInProgress
	}

	switch dlStatus {
	case v1beta1.PipelineStageInProgress:
		scanStatus = v1beta1.PipelineStageNotStarted
	case v1beta1.PipelineStageFailed:
		scanStatus = v1beta1.PipelineStageSkipped
	default:
		completed, failed = true, false
		for _, cs := range scanPod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				completed = completed && cs.State.Terminated != nil
				if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
					failed = true
				}
			}
		}

		if failed {
			scanStatus = v1beta1.PipelineStageFailed
		} else if completed {
			scanStatus = v1beta1.PipelineStageCompleted
		} else {
			scanStatus = v1beta1.PipelineStageInProgress
		}
	}
	return dlStatus, scanStatus
}

func uploaderInvocationsFromProfile(ctx context.Context, c client.Client, namespace string, uploaderRefs []v1beta1.ParameterizedLocalObjectReference) ([]resources.Invocation[v1beta1.UploaderSpec], error) {
	var uploaders []resources.Invocation[v1beta1.UploaderSpec]
	for _, uploaderRef := range uploaderRefs {
		uploader, err := resources.UploaderInvocationFromReference(ctx, c, namespace, uploaderRef)
		if err != nil {
			return nil, fmt.Errorf("unable to get uploader spec for %s/%s: %w", namespace, uploaderRef.Name, err)
		}
		uploaders = append(uploaders, uploader)
	}
	return uploaders, nil
}

func metricLabelsForPipeline(pipeline *v1beta1.Pipeline) prometheus.Labels {
	return prometheus.Labels{
		"namespace":  pipeline.Namespace,
		"downloader": pipeline.Spec.DownloaderRef.Name,
		"profile":    pipeline.Spec.ProfileRef.Name,
	}
}

func determineUploadPodStageStatuses(uploadPod *corev1.Pod) (status v1beta1.PipelineStageStatus) {
	completed, failed := true, false
	for _, cs := range uploadPod.Status.ContainerStatuses {
		completed = completed && cs.State.Terminated != nil
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			failed = true
		}
	}

	if failed {
		status = v1beta1.PipelineStageFailed
	} else if completed {
		status = v1beta1.PipelineStageCompleted
	} else {
		status = v1beta1.PipelineStageInProgress
	}

	return
}
