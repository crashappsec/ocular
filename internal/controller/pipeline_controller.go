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
	"path"
	"slices"
	"time"

	"github.com/crashappsec/ocular/internal/resources"
	ocuarlRuntime "github.com/crashappsec/ocular/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/crashappsec/ocular/api/v1beta1"
)

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	ExtractorImage      string
	ExtractorPullPolicy corev1.PullPolicy
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
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services;pods,verbs=watch;create;get;list;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The pipeline reconciler is responsible for creating and managing the scan and upload pods
// for a given pipeline. It ensures that the pods are created and updated as necessary,
// and that the status of the pipeline is updated accordingly.
// Breakdown of the reconciliation steps:
// 1. Fetch the pipeline instance
// 2. Handle finalizers
// 3. If the pipeline has a completion time, check if it needs to be deleted based on TTL
// 4. Fetch and validate the profile and downloader references
// 5. Determine if an upload pod is needed based on the profile's artifacts and uploader references
// 6. If an upload pod is needed, fetch or create the upload pod and service
// 7. Fetch or create the scan pod
// 8. Update the pipeline status accordingly based on the state of the pods
// For more details, check Reconcile and its Result here:
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("reconciling pipeline object", "name", req.Name, "namespace", req.Namespace, "req", req)

	// Fetch the Pipeline instance to be reconciled
	pipeline := &v1beta1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the pipeline has a completion time, handle post-completion logic
	if pipeline.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, pipeline)
	}

	profile := &v1beta1.Profile{}
	if err = r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.GetNamespace(),
		Name:      pipeline.Spec.ProfileRef.Name,
	}, profile); err != nil {
		return ctrl.Result{}, err
	}

	downloader := &v1beta1.Downloader{}
	if err = r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.GetNamespace(),
		Name:      pipeline.Spec.DownloaderRef.Name,
	}, downloader); err != nil {
		return ctrl.Result{}, err
	}

	uploaders, err := r.getUploaders(ctx, profile)
	if err != nil {
		l.Error(err, "error fetching uploaders for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	// In the case where no artifacts or uploaders are defined, we only need to run the scan pod
	shouldRunUploadPod := len(profile.Spec.Artifacts) > 0 && len(profile.Spec.UploaderRefs) > 0
	if !shouldRunUploadPod && !pipeline.Status.ScanPodOnly {
		pipeline.Status.ScanPodOnly = true
	}

	envVars := generateBasePipelineEnvironment(pipeline, profile, downloader)
	containerOpts := generateBaseContainerOptions(envVars)
	// extractorArgs are the arguments passed to the extractor
	// & uploaders to specify which artifacts to extract
	extractorArgs := generateExtractorArguments(downloader.Spec.MetadataFiles, profile.Spec.Artifacts)

	// generate desired upload pod, service, and scan pod
	uploadPod := r.newUploaderPod(pipeline, profile, uploaders,
		append(containerOpts,
			resources.ContainerWithAdditionalArgs(extractorArgs...),
			resources.ContainerWithWorkingDir(v1beta1.PipelineResultsDirectory),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineResultsVolumeName,
				MountPath: v1beta1.PipelineResultsDirectory,
			}),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineMetadataVolumeName,
				MountPath: v1beta1.PipelineMetadataDirectory,
			}),
		)...)

	uploadService := r.newUploadService(pipeline, uploadPod)

	scanPod := r.newScanPod(pipeline, profile, downloader,
		r.createScanExtractorContainer(pipeline, uploadService, extractorArgs),
		append(containerOpts,
			resources.ContainerWithWorkingDir(v1beta1.PipelineTargetDirectory),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineTargetVolumeName,
				MountPath: v1beta1.PipelineTargetDirectory,
			}),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineMetadataVolumeName,
				MountPath: v1beta1.PipelineMetadataDirectory,
			}),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      pipelineResultsVolumeName,
				MountPath: v1beta1.PipelineResultsDirectory,
			}))...)

	uploadPod, err = reconcilePodFromLabel(ctx, r.Client, r.Scheme, pipeline, uploadPod, []string{
		v1beta1.PipelineLabelKey,
		v1beta1.TypeLabelKey,
	})
	if err != nil {
		l.Error(err, "error reconciling upload pod for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	}

	if uploadService != nil {
		if err = resources.ReconcileChildResource[*corev1.Service](ctx, r.Client, uploadService, pipeline, r.Scheme, nil); err != nil {
			l.Error(err, "error reconciling upload service for pipeline", "name", pipeline.GetName())
			return ctrl.Result{}, err
		}
	}

	scanPod, err = reconcilePodFromLabel(ctx, r.Client, r.Scheme, pipeline, scanPod, []string{
		v1beta1.PipelineLabelKey,
		v1beta1.TypeLabelKey,
	})

	if err != nil {
		l.Error(err, "error reconciling scan pod for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	}

	// Update status to reflect pods have been created
	if pipeline.Status.StartTime == nil {
		reason, message := "ScanPodCreated", fmt.Sprintf("The scan pod %s has been created.", scanPod.Name)
		if !pipeline.Status.ScanPodOnly {
			reason = "ScanAndUploadPodCreated"
			message = fmt.Sprintf("The scan pod %s and upload pod %s have been created.", scanPod.GetName(), uploadPod.GetName())
		}
		startTime := metav1.NewTime(time.Now())
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               v1beta1.StartedConditionType,
				Status:             metav1.ConditionTrue,
				Reason:             reason,
				Message:            message,
				LastTransitionTime: startTime,
			},
		}
		pipeline.Status.StartTime = &startTime
		if err := updateStatus(ctx, r.Client, pipeline, "step", "child resources created"); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check for completion of pods and update status accordingly
	return r.handleCompletion(ctx, pipeline, scanPod, uploadPod)
}

func (r *PipelineReconciler) createScanExtractorContainer(pipeline *v1beta1.Pipeline, uploadService *corev1.Service, artifactsArgs []string) corev1.Container {
	var (
		extractorEnvVars []corev1.EnvVar
		extractorCommand = "ignore"
	)

	if !pipeline.Status.ScanPodOnly {
		extractorCommand = "extract"
		if uploadService != nil {
			extractorEnvVars = append(extractorEnvVars, corev1.EnvVar{
				Name:  v1beta1.EnvVarExtractorHost,
				Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", uploadService.Name, uploadService.Namespace, extractorPort),
			})
		}
	}

	return corev1.Container{
		Name:            "extract-artifacts",
		Image:           r.ExtractorImage,
		ImagePullPolicy: r.ExtractorPullPolicy,
		Args:            append([]string{extractorCommand}, artifactsArgs...),
		Env:             extractorEnvVars,
		RestartPolicy:   ptr.To(corev1.ContainerRestartPolicyAlways),
	}
}

func (r *PipelineReconciler) handleCompletion(ctx context.Context, pipeline *v1beta1.Pipeline, scanPod, uploadPod *corev1.Pod) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("checking for scan & upload pod completion")
	t := metav1.NewTime(time.Now())

	ttlMaxSeconds := 0
	if pipeline.Spec.TTLSecondsMaxLifetime != nil {
		ttlMaxSeconds = int(*pipeline.Spec.TTLSecondsMaxLifetime)
	}

	if ttlMaxSeconds > 0 && time.Since(pipeline.GetCreationTimestamp().Time) > time.Duration(ttlMaxSeconds)*time.Second {
		l.Info("pipeline has exceeded maximum allowed runtime, cleaning up", "maxTTL", ttlMaxSeconds)
		err := r.Delete(ctx, pipeline)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch scanPod.Status.Phase {
	case corev1.PodSucceeded:
		// scan pod completed successfully
		if pipeline.Status.ScanPodOnly {
			pipeline.Status.CompletionTime = ptr.To(t)
			pipeline.Status.Conditions = append(pipeline.Status.Conditions,
				metav1.Condition{
					Type:               v1beta1.CompletedSuccessfullyConditionType,
					Status:             metav1.ConditionTrue,
					Reason:             "ScanCompletedSuccessfully",
					Message:            "The scan pod has completed successfully.",
					LastTransitionTime: t,
				})
		} else {
			// if we have an upload pod, wait for it to complete
			switch uploadPod.Status.Phase {
			case corev1.PodSucceeded:
				pipeline.Status.CompletionTime = ptr.To(t)
				pipeline.Status.Conditions = append(pipeline.Status.Conditions,
					metav1.Condition{
						Type:               v1beta1.CompletedSuccessfullyConditionType,
						Status:             metav1.ConditionTrue,
						Reason:             "ScanAndUploadPodComplete",
						Message:            "The scan and upload pods have completed successfully.",
						LastTransitionTime: t,
					})
			case corev1.PodFailed:
				pipeline.Status.CompletionTime = ptr.To(t)
				pipeline.Status.Conditions = append(pipeline.Status.Conditions,
					metav1.Condition{
						Type:               v1beta1.CompletedSuccessfullyConditionType,
						Status:             metav1.ConditionFalse,
						Reason:             "UploadCompletedWithFailures",
						Message:            "The upload pod has failed.",
						LastTransitionTime: t,
					})
			case corev1.PodRunning, corev1.PodPending:
				// upload pod still running or pending
				// TODO(bryce): check if uploader received files or not
				return ctrl.Result{}, nil
			default:
				// upload pod in unknown state, requeue for further investigation
				l.Error(fmt.Errorf("upload pod in unknown state"), "upload pod is in an unknown state", "phase", uploadPod.Status.Phase, "name", pipeline.GetName())
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
	case corev1.PodFailed:
		pipeline.Status.CompletionTime = ptr.To(t)
		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               v1beta1.CompletedSuccessfullyConditionType,
				Status:             metav1.ConditionFalse,
				Reason:             "ScanCompletedWithFailures",
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
	case corev1.PodRunning, corev1.PodPending:
		// scan pod still running or pending
		return ctrl.Result{}, nil
	default:
		// scan pod in unknown state, requeue for further investigation
		l.Error(fmt.Errorf("scan pod in unknown state"), "scan pod is in an unknown state", "phase", scanPod.Status.Phase, "name", pipeline.GetName())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "scan pod completed")
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

func (r *PipelineReconciler) getUploaders(ctx context.Context, profile *v1beta1.Profile) (map[string]*v1beta1.Uploader, error) {
	uploaders := make(map[string]*v1beta1.Uploader, len(profile.Spec.UploaderRefs))

	for _, uploaderRef := range profile.Spec.UploaderRefs {
		var uploader v1beta1.Uploader
		err := r.Get(ctx, client.ObjectKey{
			Namespace: profile.GetNamespace(),
			Name:      uploaderRef.Name,
		}, &uploader)
		if err != nil {
			return nil, err
		}

		if _, exists := uploaders[uploader.Name]; !exists {
			uploaders[uploader.GetName()] = &uploader
		}
	}
	return uploaders, nil
}

func (r *PipelineReconciler) newUploadService(pipeline *v1beta1.Pipeline, uploadPod *corev1.Pod) *corev1.Service {
	if pipeline.Status.ScanPodOnly || uploadPod == nil {
		return nil
	}

	labels := generateChildLabels(pipeline)
	labels[v1beta1.TypeLabelKey] = v1beta1.ServiceTypeUpload
	labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
	labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name
	labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.GetName() + uploadServiceSuffix,
			Namespace: uploadPod.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				v1beta1.PipelineLabelKey: pipeline.GetName(),
				v1beta1.TypeLabelKey:     "upload",
			},
			Ports: []corev1.ServicePort{
				{Port: extractorPort, TargetPort: intstr.FromInt32(extractorPort)},
			},
			// we want to be able to connect during the init phase
			// which is before the pod is marked ready.
			PublishNotReadyAddresses: true,
		},
	}
}

func (r *PipelineReconciler) newUploaderPod(pipeline *v1beta1.Pipeline, profile *v1beta1.Profile, uploaders map[string]*v1beta1.Uploader, containerOpts ...resources.ContainerOption) *corev1.Pod {
	if pipeline.Status.ScanPodOnly {
		return nil
	}
	var (
		uploaderContainers = make([]corev1.Container, 0, len(profile.Spec.UploaderRefs))
		volumes            []corev1.Volume
	)
	for _, uploaderInvocation := range profile.Spec.UploaderRefs {
		uploader := uploaders[uploaderInvocation.Name] // should always exist since we validated during reconcile
		baseContainer := uploader.Spec.Container
		baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
			Name:  v1beta1.EnvVarUploaderName,
			Value: uploader.GetName(),
		})

		// this loop does not check for duplicate parameters NOR
		// required parameters to be set. This is done during
		// profile admission validation.

		var setParams = map[string]struct{}{}
		for _, paramDef := range uploaderInvocation.Parameters {
			setParams[paramDef.Name] = struct{}{}
			envVarName := ocuarlRuntime.ParameterToEnvironmentVariable(paramDef.Name)
			baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
				Name:  envVarName,
				Value: paramDef.Value,
			})
		}

		for _, paramDef := range uploader.Spec.Parameters {
			if _, exists := setParams[paramDef.Name]; !exists {
				if paramDef.Default != nil {
					baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
						Name:  ocuarlRuntime.ParameterToEnvironmentVariable(paramDef.Name),
						Value: *paramDef.Default,
					})
				}
			}
		}

		volumes = append(volumes, uploader.Spec.Volumes...)

		uploaderContainers = append(uploaderContainers, baseContainer)
	}

	labels := generateChildLabels(pipeline)
	labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeUpload
	labels[v1beta1.PipelineLabelKey] = pipeline.GetName()
	labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name
	labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name

	uploadPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pipeline.GetName() + uploadPodSuffix + "-",
			Namespace:    pipeline.GetNamespace(),
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: pipeline.Spec.UploadServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers:         resources.ApplyOptionsToContainers(uploaderContainers, containerOpts...),
			InitContainers: resources.ApplyOptionsToContainers([]corev1.Container{
				// Add the extractor as an init container running in receive mode
				{
					Name:  "receive-artifacts",
					Image: r.ExtractorImage,
					Args:  []string{"receive"},
					Env: []corev1.EnvVar{
						{
							Name:  v1beta1.EnvVarExtractorPort,
							Value: fmt.Sprintf("%d", extractorPort),
						},
					},
				},
			}, containerOpts...),
			Volumes: append(volumes,
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
			),
		},
	}

	return uploadPod
}

func (r *PipelineReconciler) newScanPod(pipeline *v1beta1.Pipeline, profile *v1beta1.Profile, downloader *v1beta1.Downloader,
	extractorContainer corev1.Container, containerOpts ...resources.ContainerOption) *corev1.Pod {

	volumes := append(profile.Spec.Volumes, downloader.Spec.Volumes...)

	labels := generateChildLabels(pipeline)
	labels[v1beta1.TypeLabelKey] = v1beta1.PodTypeScan
	labels[v1beta1.ProfileLabelKey] = pipeline.Spec.ProfileRef.Name
	labels[v1beta1.DownloaderLabelKey] = pipeline.Spec.DownloaderRef.Name
	labels[v1beta1.PipelineLabelKey] = pipeline.GetName()

	scanPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pipeline.GetName() + scanPodSuffix + "-",
			Namespace:    pipeline.GetNamespace(),
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: pipeline.Spec.ScanServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers:         resources.ApplyOptionsToContainers(profile.Spec.Containers, containerOpts...),
			InitContainers: resources.ApplyOptionsToContainers([]corev1.Container{
				// Add the downloader as an init container
				downloader.Spec.Container,
				// Add the extractor as a sidecar container running in extract mode
				extractorContainer,
			}, containerOpts...),
			Volumes: append(volumes,
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
			),
		},
	}

	return scanPod
}

func generateExtractorArguments(metadataFiles []string, artifacts []string) []string {
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

func generateBasePipelineEnvironment(pipeline *v1beta1.Pipeline, profile *v1beta1.Profile, downloader *v1beta1.Downloader) []corev1.EnvVar {
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
			Value: downloader.Name,
		},
		{
			Name:  v1beta1.EnvVarProfileName,
			Value: profile.Name,
		},
		{
			Name:      v1beta1.EnvVarPipelineName,
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
	if pipeline.Spec.TTLSecondsAfterFinished != nil {
		// check if we need to delete the pipeline
		finishTime := pipeline.Status.CompletionTime.Time
		ttl := time.Duration(*pipeline.Spec.TTLSecondsAfterFinished) * time.Second
		deleteTime := finishTime.Add(ttl)
		if time.Now().After(deleteTime) {
			l.Info("pipeline has exceeded its TTL, deleting", "name", pipeline.GetName(), "completionTime", pipeline.Status.CompletionTime, "ttlSecondsAfterFinished", *pipeline.Spec.TTLSecondsAfterFinished)
			if err := r.Delete(ctx, pipeline); err != nil {
				l.Error(err, "error deleting pipeline after TTL exceeded", "name", pipeline.GetName())
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else {
			l.Info("pipeline has completed, checking TTL before next reconciliation", "name", pipeline.GetName(), "completionTime", pipeline.Status.CompletionTime, "ttlSecondsAfterFinished", *pipeline.Spec.TTLSecondsAfterFinished)
			return ctrl.Result{RequeueAfter: time.Until(deleteTime)}, nil
		}
	}
	l.Info("pipeline has completed, skipping reconciliation", "name", pipeline.GetName(), "completionTime", pipeline.Status.CompletionTime)
	return ctrl.Result{}, nil
}
