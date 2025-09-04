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

	v1 "github.com/crashappsec/ocular/api/v1"
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
		For(&v1.Pipeline{}).
		Named("pipeline").
		Owns(&batchv1.Job{}, builder.MatchEveryOwner).
		Complete(r)
}

// updateAndReturn will update the [github.com/crashappsec/ocular/api/v1.Pipeline] instance
// and return a [sigs.k8s.io/controller-runtime.Result] and [error] if any.
// any additional arguments passed are called with the error log message in case of any issue updating
func (r *PipelineReconciler) updateAndReturn(ctx context.Context, pipeline *v1.Pipeline, errorMsgKeysAndValues ...any) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	if updateErr := r.Update(ctx, pipeline); updateErr != nil {
		l.Error(updateErr, "failed to update status", errorMsgKeysAndValues...)
		return ctrl.Result{}, updateErr
	}
	return ctrl.Result{}, nil
}

// updateStatus will update the status [github.com/crashappsec/ocular/api/v1.PipelineStatus] of a
// [github.com/crashappsec/ocular/api/v1.Pipeline] instance
// and return an error if any.
func (r *PipelineReconciler) updateStatus(ctx context.Context, pipeline *v1.Pipeline, errorMsgKeysAndValues ...any) error {
	l := logf.FromContext(ctx)
	if updateErr := r.Status().Update(ctx, pipeline); updateErr != nil {
		l.Error(updateErr, "failed to update status", errorMsgKeysAndValues...)
		return updateErr
	}
	return nil
}

// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=profiles;downloaders;uploaders,verbs=get;list
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipelines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ocular.crashoverride.run,resources=pipeliness/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=watch;create;get;list;update;patch;delete

func (r *PipelineReconciler) handlePostCompletion(ctx context.Context, pipeline *v1.Pipeline) (ctrl.Result, error) {
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
// 6. Fetch or create the scan job
// 7. If an upload job is needed, fetch or create the upload job and service
// 8. Update the pipeline status accordingly based on the state of the jobs
// For more details, check Reconcile and its Result here:
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	l.Info("reconciling pipeline object", "name", req.Name, "namespace", req.Namespace, "req", req)
	// Fetch the profile instance
	pipeline := &v1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // could have been deleted after reconcile request.
		}
		return ctrl.Result{}, err
	}

	finalized, err := resources.PerformFinalizer(ctx, pipeline, "pipeline.finalizers.ocular.crashoverride.run/cleanup", nil)
	if err != nil {
		l.Error(err, "error performing finalizer for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	} else if finalized {
		l.Info("pipeline has been finalized", "name", pipeline.GetName(), "finalizers", pipeline.GetFinalizers())
		return r.updateAndReturn(ctx, pipeline, "step", "finalizer")
	}

	if pipeline.Status.CompletionTime != nil {
		return r.handlePostCompletion(ctx, pipeline)
	}

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
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "profile not found")
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
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "profile is invalid")
	}

	shouldRunUploadJob := len(profile.Spec.Artifacts) > 0 && len(profile.Spec.UploaderRefs) > 0
	if !shouldRunUploadJob && !pipeline.Status.ScanJobOnly {
		pipeline.Status.ScanJobOnly = true
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "profile is valid")
	}

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
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "downloader not found")
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
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "downloader is invalid")
	}

	target := pipeline.Spec.Target

	scanJob, err := r.getScanJob(ctx, pipeline)
	if err != nil {
		l.Error(err, "error fetching scan job for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	uploadJob, err := r.getUploadJob(ctx, pipeline)
	if err != nil {
		l.Error(err, "error fetching upload job for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	uploadService, err := r.getUploadService(ctx, pipeline)
	if err != nil {
		l.Error(err, "error fetching upload service for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	// invocations are the pair of uploader definition and the run reference
	// from the profile
	uploaderInvocations, err := r.getUploaderInvocations(ctx, pipeline, profile)
	if err != nil {
		l.Error(err, "error fetching uploader invocations for pipeline", "name", req.Name)
		return ctrl.Result{}, err
	}

	envVars := []corev1.EnvVar{
		{
			Name:  v1.EnvVarOcularTargetIdentifier,
			Value: target.Identifier,
		},
		{
			Name:  v1.EnvVarOcularTargetVersion,
			Value: target.Version,
		},
		{
			Name:  v1.EnvVarOcularDownloaderName,
			Value: downloader.Name,
		},
		{
			Name:  v1.EnvVarOcularProfileName,
			Value: profile.Name,
		},
		{
			Name:  v1.EnvVarOcularPipelineName,
			Value: pipeline.Name,
		},
		{
			Name:  v1.EnvVarOcularTargetDir,
			Value: TargetDirectory,
		},
		{
			Name:  v1.EnvVarOcularResultsDir,
			Value: ResultsDirectory,
		},
	}

	// artifactArgs are the arguments passed to the extractor
	// & uploaders to specify which artifacts to extract
	artifactsArgs := []string{"--"}
	for _, artifact := range profile.Spec.Artifacts {
		artifactPath := path.Clean(artifact)
		if path.IsAbs(artifactPath) {
			artifactsArgs = append(artifactsArgs, artifactPath)
		} else {
			artifactsArgs = append(artifactsArgs, path.Join(ResultsDirectory, artifactPath))
		}
	}

	if uploadJob == nil && !pipeline.Status.ScanJobOnly {
		containerOptions := []resources.ContainerOption{
			resources.WithAdditionalEnvVars(envVars...),
			resources.WithWorkingDir(ResultsDirectory),
			resources.WithAdditionalArgs(artifactsArgs...),
			resources.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      "results",
				MountPath: ResultsDirectory,
			}),
		}

		return r.createUploaderJob(ctx, pipeline, profile, uploaderInvocations, containerOptions...)
	}

	if uploadService == nil && uploadJob != nil && !pipeline.Status.ScanJobOnly {
		uploadService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: uploadJob.GetName() + "-svc-",
				Namespace:    uploadJob.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: batchv1.SchemeGroupVersion.String(),
						Kind:       "Job",
						Name:       uploadJob.GetName(),
						UID:        uploadJob.GetUID(),
						Controller: ptr.To(true),
					},
					{
						APIVersion: pipeline.APIVersion,
						Kind:       pipeline.Kind,
						Name:       pipeline.GetName(),
						UID:        pipeline.GetUID(),
						Controller: ptr.To(false),
					},
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

		if err := r.Create(ctx, uploadService); err != nil {
			l.Error(err, "error creating upload service for pipeline", "name", pipeline.GetName())
			return ctrl.Result{}, err
		}

		pipeline.Status.UploadService = &corev1.ObjectReference{
			Name:      uploadService.Name,
			Namespace: uploadService.Namespace,
		}
		l.Info("created upload service for pipeline", "name", pipeline.GetName(), "service", uploadService.Name)

		pipeline.Status.Conditions = append(pipeline.Status.Conditions,
			metav1.Condition{
				Type:               "UploadServiceCreated",
				Status:             metav1.ConditionTrue,
				Reason:             "UploadServiceCreated",
				Message:            fmt.Sprintf("The upload service %s has been created.", uploadService.Name),
				LastTransitionTime: metav1.NewTime(time.Now()),
			})
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "upload service created")
	}

	if scanJob == nil {
		containerOptions := []resources.ContainerOption{
			resources.WithAdditionalEnvVars(envVars...),
			resources.WithWorkingDir(TargetDirectory),
			resources.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      TargetVolumeName,
				MountPath: TargetDirectory,
			}),
			resources.WithAdditionalVolumeMounts(corev1.VolumeMount{
				Name:      ResultsVolumeName,
				MountPath: ResultsDirectory,
			}),
		}
		extractor := r.createScanExtractorContainer(pipeline, uploadService, artifactsArgs)

		return r.createScanJob(ctx, pipeline, profile, downloader, extractor, containerOptions...)
	}

	return r.handleCompletion(ctx, pipeline, scanJob, uploadJob)
}

func (r *PipelineReconciler) createScanExtractorContainer(pipeline *v1.Pipeline, uploadService *corev1.Service, artifactsArgs []string) corev1.Container {
	var (
		extractorEnvVars []corev1.EnvVar
		extractorCommand = "ignore"
	)

	if !pipeline.Status.ScanJobOnly {
		extractorCommand = "extract"
		if uploadService != nil {
			extractorEnvVars = append(extractorEnvVars, corev1.EnvVar{
				Name:  v1.EnvVarExtractorHost,
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

func (r *PipelineReconciler) handleCompletion(ctx context.Context, pipeline *v1.Pipeline, scanJob, uploadJob *batchv1.Job) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("checking for scan & upload job completion")
	if !pipeline.Status.ScanJobOnly {
		if scanJob.Status.CompletionTime != nil {
			pipeline.Status.CompletionTime = scanJob.Status.CompletionTime
			pipeline.Status.Failed = ptr.To(scanJob.Status.Failed > 0)
			// return r.updateAndReturn(ctx, pipeline, "step", "scan job completed")
			return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "scan job completed")
		}
	} else if uploadJob != nil {
		if uploadJob.Status.CompletionTime != nil {
			pipeline.Status.CompletionTime = uploadJob.Status.CompletionTime
			pipeline.Status.Failed = ptr.To(uploadJob.Status.Failed > 0 && scanJob.Status.Failed > 0)
			// return r.updateAndReturn(ctx, pipeline, "step", "upload and scan job completed")
			return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "upload and scan job completed")
		}
	} else if scanJob.Status.Succeeded > 0 || scanJob.Status.Failed > 0 {
		l.Error(fmt.Errorf("scan job completed without upload job"),
			"1 or more scans have completed, but no upload job exists to capture results",
			"name", pipeline.GetName())
		pipeline.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "ScansCompletedNoUploader",
				Message:            "One or more scans have completed, but no upload job exists to capture results.",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		pipeline.Status.Failed = ptr.To(true)
		// return r.updateAndReturn(ctx, pipeline, "step", "scan complete without upload job")
		return ctrl.Result{}, r.updateStatus(ctx, pipeline, "step", "scan complete without upload job")
	}

	return ctrl.Result{}, nil
}

func (r *PipelineReconciler) getProfile(pipeline *v1.Pipeline) (*v1.Profile, bool, error) {
	if pipeline.Spec.ProfileRef == "" {
		return nil, false, nil
	}

	profile := &v1.Profile{}
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

func (r *PipelineReconciler) getDownloader(pipeline *v1.Pipeline) (*v1.Downloader, bool, error) {
	if pipeline.Spec.DownloaderRef == "" {
		return nil, false, nil
	}

	downloader := &v1.Downloader{}
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

func (r *PipelineReconciler) getUploader(ctx context.Context, pipeline *v1.Pipeline, name string) (*v1.Uploader, bool, error) {
	if name == "" {
		return nil, false, nil
	}

	uploader := &v1.Uploader{}
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

func (r *PipelineReconciler) getUploadJob(ctx context.Context, pipeline *v1.Pipeline) (*batchv1.Job, error) {
	if pipeline.Status.UploadJob == nil || pipeline.Status.ScanJobOnly {
		return nil, nil
	}

	uploadJob := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.Status.UploadJob.Namespace,
		Name:      pipeline.Status.UploadJob.Name,
	}, uploadJob); err != nil {
		if errors.IsNotFound(err) {
			pipeline.Status.UploadJob = nil // clear reference if job not found
			return nil, nil
		}
		return nil, err
	}

	return uploadJob, nil
}

func (r *PipelineReconciler) getUploadService(ctx context.Context, pipeline *v1.Pipeline) (*corev1.Service, error) {
	if pipeline.Status.UploadService == nil || pipeline.Status.ScanJobOnly {
		return nil, nil
	}

	uploadService := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.Status.UploadService.Namespace,
		Name:      pipeline.Status.UploadService.Name,
	}, uploadService); err != nil {
		if errors.IsNotFound(err) {
			pipeline.Status.UploadService = nil // clear reference if service not found
			return nil, nil
		}
		return nil, err
	}

	return uploadService, nil
}

func (r *PipelineReconciler) getScanJob(ctx context.Context, pipeline *v1.Pipeline) (*batchv1.Job, error) {
	if pipeline.Status.ScanJob == nil {
		return nil, nil
	}

	scanJob := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: pipeline.Status.ScanJob.Namespace,
		Name:      pipeline.Status.ScanJob.Name,
	}, scanJob); err != nil {
		if errors.IsNotFound(err) {
			pipeline.Status.ScanJob = nil // clear reference if job not found
			return nil, nil
		}
		return nil, err
	}

	return scanJob, nil
}

type UploaderInvocation struct {
	Uploader  *v1.Uploader
	UploadRef v1.UploaderRunRef
}

func (r *PipelineReconciler) getUploaderInvocations(ctx context.Context, pipeline *v1.Pipeline, profile *v1.Profile) (map[string]UploaderInvocation, error) {
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

func (r *PipelineReconciler) createUploaderJob(ctx context.Context, pipeline *v1.Pipeline, _ *v1.Profile, uploaderInvocations map[string]UploaderInvocation, containerOpts ...resources.ContainerOption) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	var (
		uploaders = make([]corev1.Container, 0, len(uploaderInvocations))
		volumes   []corev1.Volume
	)
	for _, invocation := range uploaderInvocations {
		baseContainer := invocation.Uploader.Spec.Container
		for paramName, paramDef := range invocation.Uploader.Spec.Parameters {
			paramValue, paramSet := invocation.UploadRef.Parameters[paramName]
			envVarName := "OCULAR_PARAM_" + paramName
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

	l.Info("creating upload job for pipeline", "name", pipeline.GetName())
	uploadJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pipeline.GetName() + "-upload-",
			Namespace:    pipeline.GetNamespace(),
			Labels: map[string]string{
				"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
				"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
				"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
				"ocular.crashoverride.run/job-type":   "uploader",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pipeline.APIVersion,
					Kind:       pipeline.Kind,
					Name:       pipeline.GetName(),
					UID:        pipeline.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: batchv1.JobSpec{
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
									Name:  v1.EnvVarExtractorPort,
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
	if err := r.Create(ctx, uploadJob); err != nil {
		l.Error(err, "error creating scan job for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	}
	pipeline.Status.UploadJob = &corev1.ObjectReference{
		Name:      uploadJob.Name,
		Namespace: uploadJob.Namespace,
	}

	if err := r.Status().Update(ctx, pipeline); err != nil {
		return ctrl.Result{}, err
	}
	l.Info("created upload job for pipeline", "name", pipeline.GetName(), "uploadJob", uploadJob)
	return ctrl.Result{}, nil
}

func (r *PipelineReconciler) createScanJob(ctx context.Context,
	pipeline *v1.Pipeline, profile *v1.Profile, downloader *v1.Downloader,
	extractorContainer corev1.Container, containerOpts ...resources.ContainerOption) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	volumes := append(profile.Spec.Volumes, downloader.Spec.Volumes...)

	l.Info("creating scan job for pipeline", "name", pipeline.GetName())
	scanJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pipeline.GetName() + "-scan-",
			Namespace:    pipeline.GetNamespace(),
			Labels: map[string]string{
				"ocular.crashoverride.run/pipeline":   pipeline.GetName(),
				"ocular.crashoverride.run/profile":    pipeline.Spec.ProfileRef,
				"ocular.crashoverride.run/downloader": pipeline.Spec.DownloaderRef,
				"ocular.crashoverride.run/job-type":   "scanner",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: pipeline.APIVersion,
					Kind:       pipeline.Kind,
					Name:       pipeline.GetName(),
					UID:        pipeline.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: batchv1.JobSpec{
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
	if err := r.Create(ctx, scanJob); err != nil {
		l.Error(err, "error creating scan job for pipeline", "name", pipeline.GetName())
		return ctrl.Result{}, err
	}
	pipeline.Status.ScanJob = &corev1.ObjectReference{
		Name:      scanJob.Name,
		Namespace: scanJob.Namespace,
	}
	l.Info("created scan job for pipeline", "name", pipeline.GetName(), "scanJob", scanJob.Name)

	reason, message := "ScanJobCreated", fmt.Sprintf("The scan job %s has been created.", scanJob.Name)
	if !pipeline.Status.ScanJobOnly {
		reason = "ScanAndUploadJobCreated"
		message = fmt.Sprintf("The scan job %s and upload job %s have been created.", scanJob.GetName(), pipeline.Status.UploadJob.Name)
	}
	startTime := metav1.NewTime(time.Now())
	pipeline.Status.Conditions = append(pipeline.Status.Conditions,
		metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: startTime,
		})
	pipeline.Status.StartTime = &startTime
	if err := r.updateStatus(ctx, pipeline, "step", "scan job created"); err != nil {
		return ctrl.Result{}, err
	}
	// TODO(bryce): possibly re-queue after some time to check on job status?
	return ctrl.Result{}, nil
}
