// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"fmt"
	"math/rand"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crashappsec/ocular/api/v1beta1"
	testutils "github.com/crashappsec/ocular/test/utils"
)

var _ = Describe("Pipeline Controller", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace        = "default"
		runtimeClassName = "kata"
		sidecarImage     = testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet) + ":latest"
		downloader       *v1beta1.Downloader
	)
	BeforeEach(func() {
		downloader = &v1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-downloader",
				Namespace: namespace,
			},
			Spec: v1beta1.DownloaderSpec{
				Container: corev1.Container{
					Name:    "downloader-container",
					Image:   "alpine:latest",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"echo Downloading...; echo $OCULAR_TARGET_IDENTIFIER > ./target.txt"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
		Expect(downloader).ToNot(BeNil())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, downloader)).To(Succeed())
	})

	When("a pipeline uses a profile with no uploaders", func() {
		var (
			profileName  = "test-profile"
			pipelineName = "test-pipeline"
			profile      = &v1beta1.Profile{}
			pipeline     = &v1beta1.Pipeline{}
		)

		BeforeEach(func() {
			profileResource := &v1beta1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      profileName,
					Namespace: namespace,
				},
				Spec: v1beta1.ProfileSpec{
					Containers: []v1beta1.ConditionalContainer{
						{
							Container: corev1.Container{
								Image:   "alpine:latest",
								Name:    "profile-container",
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
							},
						},
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "do-not-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "should-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_EMPTY",
							},
						},
					},
					Parameters: []v1beta1.ParameterDefinition{
						{
							Name:    "DEFAULT_SET",
							Default: new("1"),
						},
						{
							Name:    "DEFAULT_EMPTY",
							Default: new(""),
						},
					},
					Artifacts: []string{"results.txt"},
				},
			}
			resource := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineName,
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					DownloaderRef: v1beta1.ParameterizedLocalObjectReference{
						Name: downloader.Name,
					},
					ProfileRef: v1beta1.ParameterizedLocalObjectReference{
						Name: profileResource.Name,
						Kind: "Profile",
						Parameters: []v1beta1.ParameterSetting{
							{
								Name:  "DEFAULT_SET",
								Value: "",
							},
							{
								Name:  "DEFAULT_EMPTY",
								Value: "1",
							},
						},
					},
					Target: v1beta1.Target{
						Identifier: "https://example.com/samplefile.txt",
					},
					ScanServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					RuntimeClassName:       &runtimeClassName,
				},
			}
			Expect(k8sClient.Create(ctx, profileResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: profileName, Namespace: namespace}, profile)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pipelineName, Namespace: namespace}, pipeline)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, pipeline)).To(Succeed())
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
		})

		It("should create the pipeline", func() {
			By("Creating only a pod for the scanners")
			controllerReconciler := &PipelineReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SidecarImage:      sidecarImage,
				SidecarPullPolicy: corev1.PullIfNotPresent,
			}
			// 1st reconcile should set the status to scan pod only
			// since default is scan & upload
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())
			Expect(pipeline.Status.ScanPodOnly).To(BeTrue())

			// 2nd reconcile should create scan pod
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())

			scanPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + scanPodSuffix, Namespace: pipeline.Namespace}, scanPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelineScanPodSpec(scanPod.Spec, sidecarImage, pipeline, profile, downloader,
				[]string{"should-include", "profile-container"},
				[]corev1.EnvVar{
					{
						Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
						Value: "1",
					},
					{
						Name:  "OCULAR_PARAM_DEFAULT_SET",
						Value: "",
					}},
			)

			uploadPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + uploadPodSuffix, Namespace: pipeline.Namespace}, uploadPod)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("a pipeline uses a profile with at least one uploader", func() {
		var (
			suffix   = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
			uploader *v1beta1.Uploader
			profile  *v1beta1.Profile
			pipeline *v1beta1.Pipeline
		)

		BeforeEach(func() {
			uploader = &v1beta1.Uploader{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-uploader-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.UploaderSpec{
					Container: corev1.Container{
						Name:    "uploader-container",
						Image:   "alpine:latest",
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{"echo uploading...; cat $OCULAR_RESULTS_DIR/results.txt; echo done."},
					},
					Parameters: []v1beta1.ParameterDefinition{
						{
							Name:        "PARAM1",
							Description: "A sample parameter for the uploader",
						},
						{
							Name:        "INHERIT",
							Description: "Inheritted from Profile",
						},
						{
							Name:        "INHERIT_DEFAULT",
							Description: "Inheritted from Profile default value",
							Default:     new("uploader-default"),
						},
					},
				},
			}
			profile = &v1beta1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.ProfileSpec{
					Containers: []v1beta1.ConditionalContainer{
						{
							Container: corev1.Container{
								Image:   "alpine:latest",
								Name:    "profile-container",
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
							},
						},
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "do-not-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "should-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_EMPTY",
							},
						},
					},
					Parameters: []v1beta1.ParameterDefinition{
						{
							Name:    "DEFAULT_SET",
							Default: new("1"),
						},
						{
							Name:    "DEFAULT_EMPTY",
							Default: new(""),
						},
						{
							Name:    "PARENT",
							Default: new("profile-param"),
						},
						{
							Name:    "PARENT_DEFAULT",
							Default: new("profile-default"),
						},
					},

					Artifacts: []string{"results.txt"},
					UploaderRefs: []v1beta1.ParameterizedLocalObjectReference{
						{
							Name: uploader.Name,
							Parameters: []v1beta1.ParameterSetting{
								{
									Name:  "PARAM1",
									Value: "value1",
								},
								{
									Name: "INHERIT",
									ValueFrom: &v1beta1.ParameterSource{
										ParentParam: "PARENT",
									},
								},
								{
									Name: "INHERIT_DEFAULT",
									ValueFrom: &v1beta1.ParameterSource{
										ParentParam: "PARENT_DEFAULT",
									},
								},
							},
						},
					},
				},
			}
			pipeline = &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					DownloaderRef: v1beta1.ParameterizedLocalObjectReference{
						Name: downloader.Name,
					},
					ProfileRef: v1beta1.ParameterizedLocalObjectReference{
						Name: profile.Name,
						Kind: "Profile",
						Parameters: []v1beta1.ParameterSetting{
							{
								Name:  "DEFAULT_SET",
								Value: "",
							},
							{
								Name:  "DEFAULT_EMPTY",
								Value: "1",
							},
							{
								Name:  "PARENT",
								Value: "profile-invocation",
							},
						},
					},
					Target: v1beta1.Target{
						Identifier: "https://example.com/samplefile.txt",
					},
					ScanServiceAccountName:   testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					UploadServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					RuntimeClassName:         &runtimeClassName,
				},
			}
			Expect(k8sClient.Create(ctx, uploader)).To(Succeed())
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			Expect(k8sClient.Create(ctx, pipeline)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, pipeline)
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
			Expect(k8sClient.Delete(ctx, uploader)).To(Succeed())
		})

		It("should create the pipeline", func() {
			By("creating a scanner pod and an uploader pod")
			controllerReconciler := &PipelineReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SidecarImage:      sidecarImage,
				SidecarPullPolicy: corev1.PullIfNotPresent,
			}

			// 1st invocation creates upload pod
			// (it does not set 'scanPodOnly' so goes right into upload pod)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())

			// upload pod will be created, need to be ready for scan pod
			uploadPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + uploadPodSuffix, Namespace: pipeline.Namespace}, uploadPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelineUploadPodSpec(uploadPod.Spec, sidecarImage, pipeline, profile)

			// 2nd invocation creates service
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + uploadServiceSuffix, Namespace: pipeline.Namespace}, &corev1.Service{})).To(Succeed())

			uploadPod.Status.InitContainerStatuses = append(uploadPod.Status.InitContainerStatuses,
				corev1.ContainerStatus{
					Name:    sidecarReceiverContainerName,
					Started: new(true),
				})
			err = k8sClient.Status().Update(ctx, uploadPod)
			Expect(err).NotTo(HaveOccurred())

			// 3rd invocation will create scan pod (if upload is running)
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			scanPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + scanPodSuffix, Namespace: pipeline.Namespace}, scanPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelineScanPodSpec(scanPod.Spec, sidecarImage, pipeline, profile, downloader, []string{"should-include", "profile-container"},
				[]corev1.EnvVar{
					{
						Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
						Value: "1",
					},
					{
						Name:  "OCULAR_PARAM_DEFAULT_SET",
						Value: "",
					},
					{
						Name:  "OCULAR_PARAM_PARENT",
						Value: "profile-invocation",
					},
					{
						Name:  "OCULAR_PARAM_PARENT_DEFAULT",
						Value: "profile-default",
					},
				},
			)

			// finally, set status as running
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())
			Expect(pipeline.Status.StartTime).NotTo(BeNil())
			Expect(pipeline.Status.Phase).To(Equal(v1beta1.PipelineDownloading))
			Expect(pipeline.Status.StageStatuses.DownloadStatus).To(Equal(v1beta1.PipelineStageInProgress))
			Expect(pipeline.Status.StageStatuses.UploadStatus).To(Equal(v1beta1.PipelineStageNotStarted))
			Expect(pipeline.Status.StageStatuses.ScanStatus).To(Equal(v1beta1.PipelineStageNotStarted))
		})

	})
	When("a pipeline conditionally uses a container", func() {
		var (
			suffix   = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
			profile  *v1beta1.Profile
			pipeline *v1beta1.Pipeline
		)

		BeforeEach(func() {
			profile = &v1beta1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.ProfileSpec{
					Containers: []v1beta1.ConditionalContainer{
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "should-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: "alpine:latest",
								Name:  "do-not-include",
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_EMPTY",
							},
						},
					},
					Parameters: []v1beta1.ParameterDefinition{
						{
							Name:    "DEFAULT_SET",
							Default: new("1"),
						},
						{
							Name:    "DEFAULT_EMPTY",
							Default: new(""),
						},
					},

					Artifacts: []string{"results.txt"},
				},
			}
			pipeline = &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					DownloaderRef: v1beta1.ParameterizedLocalObjectReference{
						Name: downloader.Name,
					},
					ProfileRef: v1beta1.ParameterizedLocalObjectReference{
						Name: profile.Name,
						Kind: "Profile",
					},
					Target: v1beta1.Target{
						Identifier: "https://example.com/samplefile.txt",
					},
					ScanServiceAccountName:   testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					UploadServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			Expect(k8sClient.Create(ctx, pipeline)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, pipeline)
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
		})

		It("should create the pipeline", func() {
			By("Creating only a pod for the scanners without the conditional container")
			controllerReconciler := &PipelineReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SidecarImage:      sidecarImage,
				SidecarPullPolicy: corev1.PullIfNotPresent,
			}
			// 1st reconcile should set the status to scan pod only
			// since default is scan & upload
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())
			Expect(pipeline.Status.ScanPodOnly).To(BeTrue())

			// 2nd reconcile should create scan pod
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, pipeline)).To(Succeed())

			scanPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + scanPodSuffix, Namespace: pipeline.Namespace}, scanPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelineScanPodSpec(scanPod.Spec, sidecarImage, pipeline, profile, downloader, []string{"should-include"}, []corev1.EnvVar{
				{
					Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
					Value: "",
				},
				{
					Name:  "OCULAR_PARAM_DEFAULT_SET",
					Value: "1",
				}},
			)

			uploadPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipeline.Name + uploadPodSuffix, Namespace: pipeline.Namespace}, uploadPod)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

	})
})

func ValidatePipelineScanPodSpec(podSpec corev1.PodSpec,
	sidecarImage string,
	pipeline *v1beta1.Pipeline,
	profile *v1beta1.Profile,
	downloader *v1beta1.Downloader,
	expectedContainerNames []string,
	expectedEnvVars []corev1.EnvVar,
) {
	Expect(podSpec.InitContainers).To(HaveLen(2)) // downloader + sidecar
	// length minus 1 due to condition not being set
	Expect(podSpec.Containers).To(HaveLen(len(profile.Spec.Containers) - 1))
	containerNames := make([]string, 0, len(podSpec.Containers))
	for _, c := range podSpec.Containers {
		containerNames = append(containerNames, c.Name)
	}
	expectedNames := make([]any, 0, len(expectedContainerNames))
	for _, container := range expectedContainerNames {
		expectedNames = append(expectedNames, container)
	}
	Expect(containerNames).To(ContainElements(expectedNames...),
		fmt.Sprintf("included containers should be [%s]", strings.Join(expectedContainerNames, ",")))
	// Downloader
	Expect(podSpec.InitContainers[0].Name).To(Equal(downloader.Spec.Container.Name))
	Expect(podSpec.InitContainers[0].Image).To(Equal(downloader.Spec.Container.Image))
	Expect(podSpec.InitContainers[0].Command).To(Equal(downloader.Spec.Container.Command))
	Expect(podSpec.InitContainers[0].Args).To(Equal(downloader.Spec.Container.Args))

	// Sidecar
	Expect(podSpec.InitContainers[1].Name).To(Equal("extract-artifacts"))
	Expect(podSpec.InitContainers[1].Image).To(Equal(sidecarImage))
	Expect(podSpec.InitContainers[1].Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))
	Expect(podSpec.InitContainers[1].RestartPolicy).ToNot(BeNil())
	Expect(*podSpec.InitContainers[1].RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways)) // is a sidecar

	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.ScanServiceAccountName))
	Expect(podSpec.RuntimeClassName).To(Equal(pipeline.Spec.RuntimeClassName))
	expectedEnv := make([]any, 0, 6+len(expectedContainerNames))
	expectedEnv = append(expectedEnv,
		corev1.EnvVar{
			Name:  "OCULAR_TARGET_IDENTIFIER",
			Value: pipeline.Spec.Target.Identifier,
		},
		corev1.EnvVar{
			Name:  "OCULAR_RESULTS_DIR",
			Value: "/mnt/results",
		},
		corev1.EnvVar{
			Name:  "OCULAR_PIPELINE_NAME",
			Value: pipeline.Name,
		},
		corev1.EnvVar{
			Name: "OCULAR_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		corev1.EnvVar{
			Name:  "OCULAR_TARGET_VERSION",
			Value: pipeline.Spec.Target.Version,
		},
		corev1.EnvVar{
			Name:  "OCULAR_DOWNLOADER_NAME",
			Value: downloader.Name,
		},
	)
	for _, e := range expectedEnvVars {
		expectedEnv = append(expectedEnv, e)
	}
	for _, container := range podSpec.Containers {
		Expect(container.Env).To(ContainElements(
			expectedEnv...,
		))
	}
}

func ValidatePipelineUploadPodSpec(podSpec corev1.PodSpec,
	sidecarImage string,
	pipeline *v1beta1.Pipeline,
	profile *v1beta1.Profile) {
	Expect(podSpec.InitContainers).To(HaveLen(1)) // sidecar only
	Expect(podSpec.Containers).To(HaveLen(len(profile.Spec.UploaderRefs)))
	// sidecar
	Expect(podSpec.InitContainers[0].Name).To(Equal(sidecarReceiverContainerName))
	Expect(podSpec.InitContainers[0].Image).To(Equal(sidecarImage))
	Expect(podSpec.InitContainers[0].Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))

	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.UploadServiceAccountName))
	Expect(podSpec.RuntimeClassName).To(Equal(pipeline.Spec.RuntimeClassName))
	for _, container := range podSpec.Containers {
		Expect(container.Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))
		Expect(container.Env).To(ContainElements(
			corev1.EnvVar{
				Name:  "OCULAR_PARAM_PARAM1",
				Value: "value1",
			},
			corev1.EnvVar{
				Name:  "OCULAR_PARAM_INHERIT",
				Value: "profile-invocation",
			},
			corev1.EnvVar{
				Name:  "OCULAR_PARAM_INHERIT_DEFAULT",
				Value: "profile-default",
			},
			corev1.EnvVar{
				Name:  "OCULAR_TARGET_IDENTIFIER",
				Value: pipeline.Spec.Target.Identifier,
			},
			corev1.EnvVar{
				Name:  "OCULAR_RESULTS_DIR",
				Value: "/mnt/results",
			},
			corev1.EnvVar{
				Name:  "OCULAR_PIPELINE_NAME",
				Value: pipeline.Name,
			},
			corev1.EnvVar{
				Name: "OCULAR_POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			corev1.EnvVar{
				Name:  "OCULAR_TARGET_VERSION",
				Value: pipeline.Spec.Target.Version,
			},
		))
	}

}
