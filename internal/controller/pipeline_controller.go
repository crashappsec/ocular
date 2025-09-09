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
	"time"

	"github.com/crashappsec/ocular/internal/resources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/crashappsec/ocular/api/v1beta1"
)

const (
	ResultsVolumeName = "results"
	ResultsDirectory  = "/mnt/results"

	TargetVolumeName = "target"
	TargetDirectory  = "/mnt/target"

	ExtractorPort = 2121
)

// PipelineReconciler reconciles a Pipeline object
type PipelineReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ExtractorImage string
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Pipeline{}).
		Named("pipeline").
		Owns(&batchv1.Job{}, builder.MatchEveryOwner).
		Complete(r)
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles;downloaders;uploaders,verbs=get;list
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipeliness/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=watch;create;get;list;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The pipeline reconciler is responsible for creating and managing the scan and upload jobs
// for a given pipeline. It ensures that the jobs are created and updated as necessary,
// and that the status of the pipeline is updated accordingly.
// Breakdown of the reconciliation steps:
// 1. Fetch the pipeline instance
// 2. Handle finalizers
// 3. If the pipeline has a completion time, check if it needs to be deleted based on TTL
// 4. Fetch and validate the profile and downloader references
// 5. Determine if an upload job is needed based on the profile's artifacts and uploader references
// 6. If an upload job is needed, fetch or create the upload job and service
// 7. Fetch or create the scan job
// 8. Update the pipeline status accordingly based on the state of the jobs
// For more details, check Reconcile and its Result here:
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("reconciling pipeline object", "name", req.Name, "namespace", req.Namespace, "req", req)

	// Fetch the Pipeline instance to be reconciled
	pipeline := &v1beta1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // could have been deleted after reconcile request.
		}
		return ctrl.Result{}, err
	}

	// Handle finalizers (i.e. remove finalizer from resource if needed)
	finalized, err := resources.PerformFinalizer(ctx, pipeline, "pipeline.finalizers.ocular.crashoverride.run/cleanup", nil)
	if err != nil {
		l.Error(err, "error performing finalizer for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		l.Info("pipeline has been finalized", "name", pipeline.GetName(), "finalizers", pipeline.GetFinalizers())
		if updateErr := r.Update(ctx, pipeline); updateErr != nil {
			l.Error(updateErr, "failed to update after finalization", "name", pipeline.GetName())
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// If the pipeline has a completion time, handle post-completion logic
	if pipeline.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, pipeline)
	}

	// Check referenced profile exists and is valid
	profile, exists, err := r.getProfile(pipeline)
	if err != nil {
		l.Error(err, "error fetching profile for pipeline", "name", pipeline.GetName(), "profileRef", pipeline.Spec.ProfileRef)
		return ctrl.Result{}, err
	} else if !exists {
		l.Error(err, "profile not found for pipeline", "name", pipeline.GetName(), "profileRef", pipeline.Spec.ProfileRef)
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "ProfileNotFound",
				Message:            "The specified profile does not exist.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "profile not found")
	}

	if !profile.Status.Valid {
		l.Error(err, "profile is not valid for pipeline", "name", pipeline.GetName(), "profileRef", pipeline.Spec.ProfileRef)
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "ProfileNotValid",
				Message:            "The specified profile is not valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "profile is invalid")
	}

	// Check referenced downloader exists and is valid
	downloader, exists, err := r.getDownloader(pipeline)
	if err != nil {
		l.Error(err, "error fetching downloader for pipeline", "name", pipeline.GetName(), "downloaderRef", pipeline.Spec.DownloaderRef)
		return ctrl.Result{}, err
	} else if !exists {
		l.Error(err, "downloader not found for pipeline", "name", pipeline.GetName(), "downloaderRef", pipeline.Spec.DownloaderRef)
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "DownloaderNotFound",
				Message:            "The specified downloader does not exist.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "downloader not found")
	}

	if !downloader.Status.Valid {
		l.Error(err, "downloader is not valid for pipeline", "name", pipeline.GetName(), "downloaderRef", pipeline.Spec.DownloaderRef)
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "DownloaderNotValid",
				Message:            "The specified downloader is not valid.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "downloader is invalid")
	}

	// In the case where no artifacts or uploaders are defined, we only need to run the scan job
	shouldRunUploadJob := len(profile.Spec.Artifacts) > 0 && len(profile.Spec.UploaderRefs) > 0
	if !shouldRunUploadJob && !pipeline.Status.ScanJobOnly {
		pipeline.Status.ScanJobOnly = true
	}

	// fetch the uploader definitions referenced by the profile and tie them to their invocation parameters
	uploaderInvocations, err := r.getUploaderInvocations(ctx, pipeline, profile)
	if err != nil {
		l.Error(err, "error fetching uploader invocations for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	envVars := generateBasePipelineEnvironment(pipeline, profile, downloader)
	containerOpts := generateBaseContainerOptions(envVars)
	// artifactArgs are the arguments passed to the extractor
	// & uploaders to specify which artifacts to extract
	artifactsArgs := generateArtifactArguments(profile)

	// generate desired upload job, service, and scan job
	uploadJob := r.newUploaderJob(pipeline, profile, uploaderInvocations,
		append(containerOpts,
			resources.ContainerWithAdditionalArgs(artifactsArgs...),
			resources.ContainerWithWorkingDir(ResultsDirectory),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      ResultsVolumeName,
				MountPath: ResultsDirectory,
			}))...)

	uploadService := r.newUploadService(pipeline, uploadJob)

	scanJob := r.newScanJob(pipeline, profile, downloader, r.createScanExtractorContainer(pipeline, uploadService, artifactsArgs),
		append(containerOpts,
			resources.ContainerWithWorkingDir(TargetDirectory),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      TargetVolumeName,
				MountPath: TargetDirectory,
			}),
			resources.ContainerWithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      ResultsVolumeName,
				MountPath: ResultsDirectory,
			}))...)

	// helper function to copy status from existing job if found
	copyStatus := func(_ context.Context, _ client.Client, src *batchv1.Job, dest *batchv1.Job) error {
		dest.Status = src.Status
		return nil
	}

	if uploadJob != nil {
		if err = resources.ReconcileChildResource[*batchv1.Job](ctx, r.Client, uploadJob, pipeline, r.Scheme, copyStatus); err != nil {
			l.Error(err, "error reconciling upload job for pipeline", "name", pipeline.GetName())
			return ctrl.Result{}, err
		}
		pipeline.Status.UploadJob = &corev1.ObjectReference{
			Name:      uploadJob.Name,
			Namespace: uploadJob.Namespace,
		}
	}

	if uploadService != nil {
		if err = resources.ReconcileChildResource[*corev1.Service](ctx, r.Client, uploadService, pipeline, r.Scheme, nil); err != nil {
			l.Error(err, "error reconciling upload service for pipeline", "name", pipeline.GetName())
			return ctrl.Result{}, err
		}
		pipeline.Status.UploadService = &corev1.ObjectReference{
			Name:      uploadService.Name,
			Namespace: uploadService.Namespace,
		}
	}

	if err = resources.ReconcileChildResource[*batchv1.Job](ctx, r.Client, scanJob, pipeline, r.Scheme, copyStatus); err != nil {
		l.Error(err, "error reconciling scan job for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	}
	pipeline.Status.ScanJob = &corev1.ObjectReference{
		Name:      scanJob.Name,
		Namespace: scanJob.Namespace,
	}

	// Update status to reflect jobs have been created
	if pipeline.Status.StartTime == nil {
		reason, message := "ScanJobCreated", fmt.Sprintf("The scan job %s has been created.", scanJob.Name)
		if !pipeline.Status.ScanJobOnly {
			reason = "ScanAndUploadJobCreated"
			message = fmt.Sprintf("The scan job %s and upload job %s have been created.", scanJob.GetName(), pipeline.Status.UploadJob.Name)
		}
		startTime := metav1.NewTime(time.Now())
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
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

	// Check for completion of jobs and update status accordingly
	return r.handleCompletion(ctx, pipeline, scanJob, uploadJob)
}

func (r *PipelineReconciler) createScanExtractorContainer(pipeline *v1beta1.Pipeline, uploadService *corev1.Service, artifactsArgs []string) corev1.Container {
	var (
		extractorEnvVars []corev1.EnvVar
		extractorCommand = "ignore"
	)

	if !pipeline.Status.ScanJobOnly {
		extractorCommand = "extract"
		if uploadService != nil {
			extractorEnvVars = append(extractorEnvVars, corev1.EnvVar{
				Name:  v1beta1.EnvVarExtractorHost,
				Value: fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", uploadService.Name, uploadService.Namespace, ExtractorPort),
			})
		}
	}
	return corev1.Container{
		Name:          "extract-artifacts",
		Image:         r.ExtractorImage,
		Command:       []string{"/extractor"},
		Args:          append([]string{extractorCommand}, artifactsArgs...),
		Env:           extractorEnvVars,
		RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
	}
}

func (r *PipelineReconciler) handleCompletion(ctx context.Context, pipeline *v1beta1.Pipeline, scanJob, uploadJob *batchv1.Job) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("checking for scan & upload job completion")
	if !pipeline.Status.ScanJobOnly {
		if scanJob.Status.CompletionTime != nil {
			pipeline.Status.CompletionTime = scanJob.Status.CompletionTime
			pipeline.Status.Failed = ptr.To(scanJob.Status.Failed > 0)
			pipeline.Status.Conditions = append(pipeline.Status.Conditions,
				metav1.Condition{
					Type:               "Complete",
					Status:             metav1.ConditionTrue,
					Reason:             "ScanJobComplete",
					Message:            "The scan job has completed.",
					LastTransitionTime: metav1.NewTime(time.Now()),
				})
			return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "scan job completed")
		}
	} else if uploadJob != nil {
		if uploadJob.Status.CompletionTime != nil {
			pipeline.Status.CompletionTime = uploadJob.Status.CompletionTime
			pipeline.Status.Failed = ptr.To(uploadJob.Status.Failed > 0 && scanJob.Status.Failed > 0)
			pipeline.Status.Conditions = append(pipeline.Status.Conditions,
				metav1.Condition{
					Type:               "Complete",
					Status:             metav1.ConditionTrue,
					Reason:             "ScanAndUploadJobComplete",
					Message:            "Both the scan and upload jobs have completed.",
					LastTransitionTime: metav1.NewTime(time.Now()),
				})
			return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "upload and scan job completed")
		}
	} else if scanJob.Status.Succeeded > 0 || scanJob.Status.Failed > 0 {
		l.Error(fmt.Errorf("scan job completed without upload job"),
			"1 or more scans have completed, but no upload job exists to capture results",
			"name", pipeline.GetName())
		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               "Complete",
				Status:             metav1.ConditionFalse,
				Reason:             "ScansCompletedNoUploader",
				Message:            "One or more scans have completed, but no upload job exists to capture results.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			})

		pipeline.Status.Failed = ptr.To(true)
		// return r.updateAndReturn(ctx, pipeline, "step", "scan complete without upload job")
		return ctrl.Result{}, updateStatus(ctx, r.Client, pipeline, "step", "scan complete without upload job")
	}

	return ctrl.Result{}, nil
}

func (r *PipelineReconciler) getProfile(pipeline *v1beta1.Pipeline) (*v1beta1.Profile, bool, error) {
	if pipeline.Spec.ProfileRef == "" {
		return nil, false, nil
	}

	profile := &v1beta1.Profile{}
	if err := r.Get(context.Background(), client.ObjectKey{
		Namespace: pipeline.GetNamespace(),
		Name:      pipeline.Spec.ProfileRef,
	}, profile); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return profile, true, nil
}

func (r *PipelineReconciler) getDownloader(pipeline *v1beta1.Pipeline) (*v1beta1.Downloader, bool, error) {
	if pipeline.Spec.DownloaderRef == "" {
		return nil, false, nil
	}

	downloader := &v1beta1.Downloader{}
	if err := r.Get(context.Background(), client.ObjectKey{
		Namespace: pipeline.GetNamespace(),
		Name:      pipeline.Spec.DownloaderRef,
	}, downloader); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return downloader, true, nil
}

func (r *PipelineReconciler) getUploader(ctx context.Context, pipeline *v1beta1.Pipeline, name string) (*v1beta1.Uploader, bool, error) {
	if name == "" {
		return nil, false, nil
	}

	uploader := &v1beta1.Uploader{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.GetNamespace(),
		Name:      name,
	}, uploader); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return uploader, true, nil
}

type UploaderInvocation struct {
	Uploader  *v1beta1.Uploader
	UploadRef v1beta1.UploaderRunRef
}

func (r *PipelineReconciler) getUploaderInvocations(ctx context.Context, pipeline *v1beta1.Pipeline, profile *v1beta1.Profile) (map[string]UploaderInvocation, error) {
	l := logf.FromContext(ctx)
	uploaderInvocations := make(map[string]UploaderInvocation, len(profile.Spec.UploaderRefs))

	for _, uploaderRef := range profile.Spec.UploaderRefs {
		uploader, exists, err := r.getUploader(ctx, pipeline, uploaderRef.Name)
		if err != nil || !exists {
			// we can just error here since check for existence is done during reconcile of profile
			l.Error(err, "error fetching uploader for pipeline", "name", pipeline.GetName(), "uploaderRef", uploaderRef)
			return nil, err
		}

		uploaderInvocations[uploaderRef.Name] = UploaderInvocation{
			Uploader:  uploader,
			UploadRef: uploaderRef,
		}
	}
	return uploaderInvocations, nil
}

func (r *PipelineReconciler) newUploadService(pipeline *v1beta1.Pipeline, uploadJob *batchv1.Job) *corev1.Service {
	if pipeline.Status.ScanJobOnly || uploadJob == nil {
		return nil
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uploadJob.GetName() + "-upload-svc",
			Namespace: uploadJob.Namespace,
			Labels: map[string]string{
				"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
				"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
				"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
				"ocular.crashoverride.run/job-type":   "uploader",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"job-name": uploadJob.Name,
			},
			Ports: []corev1.ServicePort{
				{Port: ExtractorPort, TargetPort: intstr.FromInt32(ExtractorPort)},
			},
			// we want to be able to connect during the init phase
			// which is before the pod is marked ready.
			PublishNotReadyAddresses: true,
		},
	}
}

func (r *PipelineReconciler) newUploaderJob(pipeline *v1beta1.Pipeline, _ *v1beta1.Profile, uploaderInvocations map[string]UploaderInvocation, containerOpts ...resources.ContainerOption) *batchv1.Job {
	if pipeline.Status.ScanJobOnly {
		return nil
	}
	var (
		uploaders = make([]corev1.Container, 0, len(uploaderInvocations))
		volumes   []corev1.Volume
	)
	for _, invocation := range uploaderInvocations {
		baseContainer := invocation.Uploader.Spec.Container
		for paramName, paramDef := range invocation.Uploader.Spec.Parameters {
			paramValue, paramSet := invocation.UploadRef.Parameters[paramName]
			envVarName := v1beta1.ParameterToEnvironmentVariable(paramName)
			if paramSet {
				baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
					Name:  envVarName,
					Value: paramValue,
				})
			} else {
				// if not set, and default exists, use default
				if paramDef.Default != "" {
					baseContainer.Env = append(baseContainer.Env, corev1.EnvVar{
						Name:  envVarName,
						Value: paramDef.Default,
					})
				}
			}
		}
		volumes = append(volumes, invocation.Uploader.Spec.Volumes...)

		uploaders = append(uploaders, baseContainer)
	}

	uploadJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.GetName() + "-upload",
			Namespace: pipeline.GetNamespace(),
			Labels: map[string]string{
				"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
				"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
				"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
				"ocular.crashoverride.run/job-type":   "uploader",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](0),
			Parallelism:  ptr.To[int32](1),
			Completions:  ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
						"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
						"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
						"ocular.crashoverride.run/job-type":   "uploader",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    resources.ApplyOptionsToContainers(uploaders, containerOpts...),
					InitContainers: resources.ApplyOptionsToContainers([]corev1.Container{
						// Add the extractor as an init container running in receive mode
						{
							Name:    "receive-artifacts",
							Image:   r.ExtractorImage,
							Command: []string{"/extractor"},
							Args:    []string{"receive"},
							Env: []corev1.EnvVar{
								{
									Name:  v1beta1.EnvVarExtractorPort,
									Value: fmt.Sprintf("%d", ExtractorPort),
								},
							},
						},
					}, containerOpts...),
					Volumes: append(volumes,
						// add shared volume for target and results
						corev1.Volume{Name: "target",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						}, corev1.Volume{Name: "results",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						}),
				},
			},
		},
	}
	return uploadJob
}

func (r *PipelineReconciler) newScanJob(pipeline *v1beta1.Pipeline, profile *v1beta1.Profile, downloader *v1beta1.Downloader,
	extractorContainer corev1.Container, containerOpts ...resources.ContainerOption) *batchv1.Job {

	volumes := append(profile.Spec.Volumes, downloader.Spec.Volumes...)

	scanJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.GetName() + "-scan",
			Namespace: pipeline.GetNamespace(),
			Labels: map[string]string{
				"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
				"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
				"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
				"ocular.crashoverride.run/job-type":   "scanner",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](0),
			Parallelism:  ptr.To[int32](1),
			Completions:  ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
						"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
						"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
						"ocular.crashoverride.run/job-type":   "scanner",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    resources.ApplyOptionsToContainers(profile.Spec.Containers, containerOpts...),
					InitContainers: resources.ApplyOptionsToContainers([]corev1.Container{
						// Add the downloader as an init container
						downloader.Spec.Container,
						// Add the extractor as a sidecar container running in extract mode
						extractorContainer,
					}, containerOpts...),
					Volumes: append(volumes,
						corev1.Volume{Name: TargetVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						}, corev1.Volume{Name: ResultsVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						}),
				},
			},
		},
	}
	return scanJob
}

func generateArtifactArguments(profile *v1beta1.Profile) []string {
	artifactsArgs := []string{"--"}
	for _, artifact := range profile.Spec.Artifacts {
		artifactPath := path.Clean(artifact)
		if path.IsAbs(artifactPath) {
			artifactsArgs = append(artifactsArgs, artifactPath)
		} else {
			artifactsArgs = append(artifactsArgs, path.Join(ResultsDirectory, artifactPath))
		}
	}
	return artifactsArgs
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
			Value: TargetDirectory,
		},
		{
			Name:  v1beta1.EnvVarResultsDir,
			Value: ResultsDirectory,
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
