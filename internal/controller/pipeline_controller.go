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
	"strings"
	"time"

	"github.com/crashappsec/ocular/internal/containers"
	"github.com/crashappsec/ocular/internal/resources"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
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
		// nolint:goconst
		[]string{"profile", "downloader", "namespace", "phase"},
	)
	pipelinesRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipelines_active",
			Help: "Number of ocular pipelines running currently",
		},
		[]string{"profile", "downloader", "namespace"},
	)
	pipelinePodsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_pods_total",
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
		pipelinePodsCreated,
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
		Owns(&corev1.Pod{}, builder.WithPredicates(podStateChangedPredicate)).
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
// 3. Fetch or create pipeline pod
// 4. Continually Update the pipeline status accordingly based on the state of the pods
// 5. Once completed, await TTL if set
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
		l.Info("pipeline deleted before finalizer was removed, updating metrics")
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

	scanPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pipelineResourcePrefix + pipeline.GetName(), Namespace: pipeline.GetNamespace()}}
	scanPodOp, err := controllerutil.CreateOrUpdate(ctx, r.Client, scanPod, func() error {
		return r.populateScanPod(scanPod, pipeline, profile, downloader, uploaders)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to generate new scan pod: %w", err)
	}

	l = l.WithValues("scan-pod", scanPod.Name)

	switch scanPodOp {
	case controllerutil.OperationResultCreated:
		pipelinePodsCreated.With(metricLabelsForPipeline(pipeline)).Inc()
		fallthrough
	case controllerutil.OperationResultUpdated:
		l.Info("scan pod was created or modified", "op", scanPodOp)
	}

	if !pipeline.Status.StartTime.IsZero() {
		if !controllerutil.ContainsFinalizer(pipeline, metricsFinalizer) {
			patch := client.MergeFrom(pipeline.DeepCopy())
			controllerutil.AddFinalizer(pipeline, metricsFinalizer)
			if err := patchResource(ctx, r.Client, pipeline, patch); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add metrics finalizer: %w", err)
			}
			l.Info("pipeline starting, incrementing pipeline running count")
			pipelinesRunning.With(metricLabelsForPipeline(pipeline)).Inc()
			return ctrl.Result{Priority: new(25)}, nil
		}

		// Check for completion of pods and update status accordingly
		return r.handleCompletion(logf.IntoContext(ctx, l), pipeline, scanPod)
	}

	// Update status to reflect pods have been created
	l.Info("marking pipeline as started")
	startTime := scanPod.CreationTimestamp
	patch := client.MergeFrom(pipeline.DeepCopy())
	reason, message := "ScanPodSuccessfullyCreated", fmt.Sprintf("The scan pod %s has been created.", scanPod.Name)
	pipeline.Status.Conditions = append(pipeline.Status.Conditions, metav1.Condition{
		Type:               v1beta1.PipelineScanPodCreatedConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: startTime.Rfc3339Copy(),
	})

	pipeline.Status.StartTime = &startTime
	pipeline.Status.Phase = v1beta1.PipelineDownloading
	pipeline.Status.StageStatuses.DownloadStatus = v1beta1.PipelineStageInProgress
	pipeline.Status.StageStatuses.ScanStatus = v1beta1.PipelineStageNotStarted
	err = patchStatus(logf.IntoContext(ctx, l), r.Client, pipeline, patch)
	return ctrl.Result{}, err

}

func (r *PipelineReconciler) handleCompletion(ctx context.Context, pipeline *v1beta1.Pipeline, scanPod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("checking for scan & upload pod completion")

	t := metav1.NewTime(time.Now())
	patch := client.MergeFrom(pipeline.DeepCopy())

	switch scanPod.Status.Phase {
	case corev1.PodSucceeded:
		// scan pod completed successfully
		pipeline.Status.StageStatuses.DownloadStatus = v1beta1.PipelineStageCompleted
		pipeline.Status.StageStatuses.ScanStatus = v1beta1.PipelineStageCompleted
		if pipeline.Status.StageStatuses.UploadStatus != v1beta1.PipelineStageSkipped {
			pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageCompleted
		}
		pipeline.Status.Phase = v1beta1.PipelineSucceeded
		pipeline.Status.CompletionTime = new(t)
		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               v1beta1.PipelineCompletedSuccessfullyConditionType,
				Status:             metav1.ConditionTrue,
				Reason:             "PodCompletedSuccessfully",
				Message:            "The pipeline has completed successfully.",
				LastTransitionTime: t,
			})
	case corev1.PodFailed:
		downloaderStatus, scanStatus, uploadStatus := determineScanPodStageStatuses(scanPod)
		pipeline.Status.StageStatuses.DownloadStatus = downloaderStatus
		pipeline.Status.StageStatuses.ScanStatus = scanStatus
		if pipeline.Status.StageStatuses.UploadStatus != v1beta1.PipelineStageSkipped {
			pipeline.Status.StageStatuses.UploadStatus = uploadStatus
		}
		pipeline.Status.Phase = v1beta1.PipelineFailed
		pipeline.Status.CompletionTime = new(t)
		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               v1beta1.CompletedSuccessfullyConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "PodTerminatedWithFailures",
				Message:            "The pipeline pod has failed.",
				LastTransitionTime: t,
			})
	case corev1.PodRunning:
		downloaderStatus, scanStatus, uploadStatus := determineScanPodStageStatuses(scanPod)
		pipeline.Status.StageStatuses = v1beta1.PipelineStageStatuses{
			DownloadStatus: downloaderStatus,
			ScanStatus:     scanStatus,
		}
		if pipeline.Status.StageStatuses.UploadStatus != v1beta1.PipelineStageSkipped {
			pipeline.Status.StageStatuses.UploadStatus = uploadStatus
		}

		if downloaderStatus == v1beta1.PipelineStageInProgress {
			pipeline.Status.Phase = v1beta1.PipelineDownloading
		} else if scanStatus == v1beta1.PipelineStageInProgress {
			pipeline.Status.Phase = v1beta1.PipelineScanning
		} else if pipeline.Status.StageStatuses.UploadStatus != v1beta1.PipelineStageSkipped {
			pipeline.Status.Phase = v1beta1.PipelineUploading
		}
	case corev1.PodPending:
		pipeline.Status.StageStatuses.DownloadStatus = v1beta1.PipelineStageNotStarted
		pipeline.Status.StageStatuses.ScanStatus = v1beta1.PipelineStageNotStarted
		if pipeline.Status.StageStatuses.UploadStatus != v1beta1.PipelineStageSkipped {
			pipeline.Status.StageStatuses.UploadStatus = v1beta1.PipelineStageNotStarted
		}
		pipeline.Status.Phase = v1beta1.PipelineSucceeded
	default:
		// scan pod in unknown state, requeue for further investigation
		l.Error(fmt.Errorf("scan pod in unknown state"), "scan pod is in an unknown state", "phase", scanPod.Status.Phase, "name", pipeline.GetName())
		pipeline.Status.Phase = v1beta1.PipelineStateUnknown
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	l = l.WithValues("phase", pipeline.Status.Phase)
	err := patchStatus(logf.IntoContext(ctx, l), r.Client, pipeline, patch)
	return ctrl.Result{}, err
}

func (r *PipelineReconciler) populateScanPod(
	pod *corev1.Pod,
	pipeline *v1beta1.Pipeline,
	profile resources.Invocation[v1beta1.ProfileSpec],
	downloader resources.Invocation[v1beta1.DownloaderSpec],
	uploaders []resources.Invocation[v1beta1.UploaderSpec],
) error {
	// only edit pod spec if not created yet
	// since once created, spec cant really be modified
	if pod.CreationTimestamp.IsZero() {

		targetVolume := corev1.Volume{
			Name: pipelineTargetVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		resultsVolume := corev1.Volume{
			Name: pipelineResultsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		metadataVolume := corev1.Volume{
			Name: pipelineMetadataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		processVolume := corev1.Volume{
			Name: runtimeProcVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		binaryVolume := corev1.Volume{
			Name: runtimeBinaryVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

		pod.Spec.Volumes = []corev1.Volume{
			targetVolume,
			binaryVolume,
			resultsVolume,
			metadataVolume,
			processVolume,
		}

		envVars := generateBasePipelineEnvironment(pipeline)

		baseContainerOptions := []containers.Option{
			containers.WithAdditionalEnvVars(envVars...),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      metadataVolume.Name,
				MountPath: pipelineMetadataDirectory,
			}),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      resultsVolume.Name,
				MountPath: pipelineResultsDirectory,
			}),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      processVolume.Name,
				MountPath: processDirectory,
			}),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      binaryVolume.Name,
				MountPath: binaryDirectory,
			}),
		}

		/* init containers (downloader + sidecar) */

		downloaderOptions := []containers.Option{
			containers.WithWorkingDir(pipelineTargetDirectory),
			containers.WithParameters(downloader.Spec.Parameters, pipeline.Spec.DownloaderRef.Parameters, nil),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      targetVolume.Name,
				MountPath: pipelineTargetDirectory,
			}),
			containers.WithNamePrefix(downloadContainerPrefix),
		}
		downloaderContainer := containers.ApplyOptionsTo(
			downloader.Spec.Container,
			downloaderOptions...,
		)

		initContainers := containers.ApplyOptionsToAll(
			[]corev1.Container{
				// sidecar init to copy wrapper binary into runtime direction
				{
					Name:            sidecarInitContainerName,
					Image:           r.SidecarImage,
					ImagePullPolicy: r.SidecarPullPolicy,
					Args:            []string{"init"},
					Resources:       *sidecarInitResourceRequirements.DeepCopy(),
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: new(true),
					},
				},
				// Add the downloader as an init container
				downloaderContainer,
			}, baseContainerOptions...,
		)

		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, downloader.Spec.ImagePullSecrets...)
		pod.Spec.Volumes = append(pod.Spec.Volumes, downloader.Spec.Volumes...)

		/* scanner containers */

		scannerOptions := append(baseContainerOptions,
			containers.WrapCommand(sidecarBinaryPath, "scanner"),
			containers.WithWorkingDir(pipelineTargetDirectory),
			containers.WithParameters(profile.Spec.Parameters, pipeline.Spec.ProfileRef.Parameters, nil),
			containers.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      targetVolume.Name,
				MountPath: pipelineTargetDirectory,
			}),
			containers.WithNamePrefix(scanContainerPrefix),
		)
		scannerContainers := containers.ApplyOptionsToAll(
			containers.FilterConditionalContainers(profile.Spec.Containers, profile.Spec.Parameters, pipeline.Spec.ProfileRef.Parameters),
			scannerOptions...,
		)

		scanContainerNames := make([]string, len(scannerContainers))
		for i, c := range scannerContainers {
			scanContainerNames[i] = c.Name
		}

		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, profile.Spec.ImagePullSecrets...)
		pod.Spec.Volumes = append(pod.Spec.Volumes, profile.Spec.Volumes...)

		/* uploader containers */

		// aritfactArgs are the arguments passed to
		// uploaders to specify which artifacts to extract
		artifactArgs := generateArtifactArguments(downloader.Spec.MetadataFiles, profile.Spec.Artifacts)

		uploaderContainers := make([]corev1.Container, 0, len(uploaders))
		uploaderLabels, uploaderAnnotations := make(map[string]string), make(map[string]string)
		parentParams := resources.ParseParameters(profile.Spec.Parameters, profile.Parameters, nil)
		for _, invocation := range uploaders {
			baseContainer := invocation.Spec.Container

			maps.Copy(uploaderLabels, invocation.Metadata.GetLabels())
			maps.Copy(uploaderAnnotations, invocation.Metadata.GetAnnotations())

			uploaderContainers = append(uploaderContainers,
				containers.ApplyOptionsTo(
					baseContainer,
					containers.WithParameters(invocation.Spec.Parameters, invocation.Parameters, parentParams),
					containers.WithAdditionalEnvVars(corev1.EnvVar{
						Name:  v1beta1.EnvVarUploaderName,
						Value: invocation.Metadata.Name,
					}),
				),
			)
			pod.Spec.Volumes = append(pod.Spec.Volumes, invocation.Spec.Volumes...)
			pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, invocation.Spec.ImagePullSecrets...)
		}

		uploaderOpts := append(baseContainerOptions,
			containers.WrapCommand(sidecarBinaryPath, "await-scanners"),
			containers.WithAdditionalArgs(artifactArgs...),
			containers.WithWorkingDir(pipelineResultsDirectory),
			containers.WithAdditionalEnvVars(corev1.EnvVar{
				Name:  v1beta1.EnvVarScanContainerNames,
				Value: strings.Join(scanContainerNames, ","),
			}),
			containers.WithNamePrefix(uploadContainerPrefix),
		)

		uploaderContainers = containers.ApplyOptionsToAll(
			// TODO: conditional reference
			uploaderContainers,
			uploaderOpts...,
		)

		pod.Spec.ServiceAccountName = pipeline.Spec.ServiceAccountName
		pod.Spec.RuntimeClassName = pipeline.Spec.RuntimeClassName
		pod.Spec.RestartPolicy = corev1.RestartPolicyNever
		pod.Spec.InitContainers = containers.ApplyStandardOptions(initContainers)
		pod.Spec.Containers = containers.ApplyStandardOptions(
			append(scannerContainers, uploaderContainers...),
		)

		pod.Spec.Resources = pipeline.Spec.Resources.DeepCopy()
		pod.Spec.ImagePullSecrets = slices.CompactFunc(
			pod.Spec.ImagePullSecrets,
			func(s1, s2 corev1.LocalObjectReference) bool { return s1.Name == s2.Name },
		)

		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		maps.Copy(pod.Labels, resources.PropagateMetadata(downloader.Metadata.GetLabels(), profile.Metadata.GetLabels(), uploaderLabels))
		pod.Labels[v1beta1.TypeLabelKey] = v1beta1.PipelinePodType
		pod.Labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
		pod.Labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name
		pod.Labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		maps.Copy(pod.Annotations, resources.PropagateMetadata(downloader.Metadata.GetAnnotations(), profile.Metadata.GetAnnotations(), uploaderAnnotations))

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
		{
			Name:  v1beta1.EnvVarProcessDir,
			Value: processDirectory,
		},
		{
			Name:  v1beta1.EnvVarSidecarPath,
			Value: sidecarBinaryPath,
		},
	}

}

func (r *PipelineReconciler) handlePostCompletion(ctx context.Context, pipeline *v1beta1.Pipeline) (ctrl.Result, error) {
	l := logf.FromContext(ctx).WithValues(
		"pipeline", pipeline.Name, "namespace", pipeline.Namespace,
		"profile", pipeline.Spec.ProfileRef.Name, "downloader", pipeline.Spec.DownloaderRef.Name,
		"target", pipeline.Spec.Target, "phase", pipeline.Status.Phase,
		"completion-time", pipeline.Status.CompletionTime, "start-time", pipeline.Status.StartTime,
	)
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
		l.Info("pipeline metrics updated with completion")
	}
	if pipeline.Spec.TTLSecondsAfterFinished == nil {
		l.Info("pipeline has completed")
		return ctrl.Result{}, nil
	}

	ttl := time.Duration(*pipeline.Spec.TTLSecondsAfterFinished) * time.Second
	wait := time.Until(pipeline.Status.CompletionTime.Add(ttl))
	if wait <= 0 {
		l.Info("pipeline has exceeded its TTL, deleting",
			"ttl", ttl)
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, pipeline))
	}

	l.Info("pipeline has completed, checking TTL before next reconciliation",
		"ttl", ttl.Seconds(),
		"requeue-after", wait.String(),
	)

	return ctrl.Result{RequeueAfter: wait}, nil
}

func determineScanPodStageStatuses(scanPod *corev1.Pod) (download, scan, upload v1beta1.PipelineStageStatus) {
	for _, cs := range scanPod.Status.InitContainerStatuses {
		if cs.Name != sidecarInitContainerName {
			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				download = v1beta1.PipelineStageCompleted
			} else if cs.State.Terminated != nil {
				download = v1beta1.PipelineStageFailed
			} else {
				download = v1beta1.PipelineStageInProgress
			}
		}

	}

	switch download {
	case v1beta1.PipelineStageInProgress:
		scan = v1beta1.PipelineStageNotStarted
	case v1beta1.PipelineStageFailed:
		scan = v1beta1.PipelineStageSkipped
	default:
		scan = v1beta1.PipelineStageCompleted
		for _, cs := range scanPod.Status.ContainerStatuses {
			if strings.HasPrefix(cs.Name, scanContainerPrefix) {
				if cs.State.Terminated == nil {
					scan = v1beta1.PipelineStageInProgress
					break
				} else if cs.State.Terminated.ExitCode != 0 {
					scan = v1beta1.PipelineStageFailed
					break
				}
			}

		}
	}

	switch scan {
	case v1beta1.PipelineStageInProgress:
		upload = v1beta1.PipelineStageNotStarted
	case v1beta1.PipelineStageSkipped:
		upload = v1beta1.PipelineStageSkipped
	default:
		for _, cs := range scanPod.Status.ContainerStatuses {
			if strings.HasPrefix(cs.Name, uploadContainerPrefix) {
				if cs.State.Terminated == nil {
					upload = v1beta1.PipelineStageInProgress
					break
				} else if cs.State.Terminated.ExitCode != 0 {
					upload = v1beta1.PipelineStageFailed
					break
				}
			}

		}
	}
	return download, scan, upload
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
