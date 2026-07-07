// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"math/rand"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crashappsec/ocular/api/v1beta1"
	testutils "github.com/crashappsec/ocular/test/utils"
)

var _ = Describe("Pipeline Controller", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace             = testNamespace
		runtimeClassName      = "kata"
		downloadContainerName = "download-container"
		sidecarImage          = testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet) + ":latest"
		downloader            *v1beta1.Downloader
	)
	BeforeEach(func() {
		downloader = &v1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-downloader",
				Namespace: namespace,
			},
			Spec: v1beta1.DownloaderSpec{
				Container: corev1.Container{
					Name:  downloadContainerName,
					Image: testImage,
					// nolint:goconst
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
			profileName               = "test-profile"
			pipelineName              = "test-pipeline"
			scannerContainerName      = "profile-container"
			doNotIncludeContainerName = "do-not-include"
			includeContainerName      = "should-include"
			profile                   = &v1beta1.Profile{}
			pipeline                  = &v1beta1.Pipeline{}
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
								Image:   testImage,
								Name:    scannerContainerName,
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
							},
						},
						{
							Container: corev1.Container{
								Image: testImage,
								// nolint:goconst
								Name: doNotIncludeContainerName,
							},
							IncludeIf: &v1beta1.ContainerCondition{
								// nolint:goconst
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: testImage,
								Name:  includeContainerName,
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
					ServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					RuntimeClassName:   &runtimeClassName,
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
			By("Creating a pod")
			controllerReconciler := &PipelineReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SidecarImage:      sidecarImage,
				SidecarPullPolicy: corev1.PullIfNotPresent,
			}

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

			scanPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipelineResourcePrefix + pipeline.Name, Namespace: pipeline.Namespace}, scanPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelinePodSpec(
				scanPod.Spec,
				sidecarImage,
				pipeline,
				profile,
				downloader,
				func(c corev1.Container) {
					Expect(c.Name).To(Equal(downloadContainerPrefix + downloadContainerName))
					Expect(c.Image).To(Equal(testImage))
				},
				map[string]func(_ corev1.Container){
					scannerContainerName: func(c corev1.Container) {
						Expect(c.Name).To(Equal(scanContainerPrefix + scannerContainerName))
						Expect(c.Image).To(Equal(testImage))
					},
					includeContainerName: func(c corev1.Container) {
						Expect(c.Env).To(ContainElements(corev1.EnvVar{
							Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
							Value: "1",
						}, corev1.EnvVar{
							Name:  "OCULAR_PARAM_DEFAULT_SET",
							Value: "",
						}))
					},
				},
				nil,
			)
		})
	})

	When("a pipeline uses a profile with at least one uploader", func() {
		var (
			suffix                    = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
			uploaderContainerName     = "uploader-container"
			scannerContainerName      = "profile-container"
			doNotIncludeContainerName = "do-not-include"
			includeContainerName      = "should-include"
			uploader                  *v1beta1.Uploader
			profile                   *v1beta1.Profile
			pipeline                  *v1beta1.Pipeline
		)

		BeforeEach(func() {
			uploader = &v1beta1.Uploader{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-uploader-" + suffix,
					Namespace: namespace,
				},
				Spec: v1beta1.UploaderSpec{
					Container: corev1.Container{
						Name:    uploaderContainerName,
						Image:   testImage,
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
								Image:   testImage,
								Name:    scannerContainerName,
								Command: []string{"/bin/sh", "-c"},
								Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
							},
						},
						{
							Container: corev1.Container{
								Image: testImage,
								Name:  doNotIncludeContainerName,
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: testImage,
								Name:  includeContainerName,
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
					ServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					RuntimeClassName:   &runtimeClassName,
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
			By("creating a pipeline pod, and include uploaders")
			controllerReconciler := &PipelineReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SidecarImage:      sidecarImage,
				SidecarPullPolicy: corev1.PullIfNotPresent,
			}

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

			pipelinePod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipelineResourcePrefix + pipeline.Name, Namespace: pipeline.Namespace}, pipelinePod)
			Expect(err).NotTo(HaveOccurred())
			expectedScannerParams := []any{
				corev1.EnvVar{
					Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
					Value: "1",
				}, corev1.EnvVar{
					Name:  "OCULAR_PARAM_DEFAULT_SET",
					Value: "",
				}, corev1.EnvVar{
					Name:  "OCULAR_PARAM_PARENT",
					Value: "profile-invocation",
				}, corev1.EnvVar{
					Name:  "OCULAR_PARAM_PARENT_DEFAULT",
					Value: "profile-default",
				},
			}
			expectedUploaderParams := []any{
				corev1.EnvVar{
					Name:  "OCULAR_PARAM_PARAM1",
					Value: "value1",
				}, corev1.EnvVar{
					Name:  "OCULAR_PARAM_INHERIT",
					Value: "profile-invocation",
				}, corev1.EnvVar{
					Name:  "OCULAR_PARAM_INHERIT_DEFAULT",
					Value: "profile-default",
				},
			}

			ValidatePipelinePodSpec(
				pipelinePod.Spec,
				sidecarImage,
				pipeline,
				profile,
				downloader,
				func(c corev1.Container) {
					Expect(c.Name).To(Equal(downloadContainerPrefix + downloadContainerName))
					Expect(c.Image).To(Equal(testImage))
				},
				map[string]func(corev1.Container){
					scannerContainerName: func(c corev1.Container) {
						Expect(c.Name).To(Equal(scanContainerPrefix + scannerContainerName))
						Expect(c.Image).To(Equal(testImage))
						Expect(c.Env).To(ContainElements(expectedScannerParams...))
					},
					includeContainerName: func(c corev1.Container) {

						Expect(c.Env).To(ContainElements(expectedScannerParams...))
					},
				},
				map[string]func(corev1.Container){
					uploaderContainerName: func(c corev1.Container) {
						Expect(c.Env).To(ContainElements(expectedUploaderParams...))
						Expect(c.Env).To(ContainElement(corev1.EnvVar{
							Name:  v1beta1.EnvVarUploaderName,
							Value: uploader.Name,
						}))
					},
				},
			)

			// TODO: test status
			// _, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
			// 	NamespacedName: types.NamespacedName{
			// 		Name:      pipeline.Name,
			// 		Namespace: pipeline.Namespace,
			// 	},
			// })
			// Expect(err).NotTo(HaveOccurred())
			// Expect(k8sClient.Get(ctx, types.NamespacedName{
			// 	Name:      pipeline.Name,
			// 	Namespace: pipeline.Namespace,
			// }, pipeline)).To(Succeed())
			// Expect(pipeline.Status.StartTime).NotTo(BeNil())
			// Expect(pipeline.Status.Phase).To(Equal(v1beta1.PipelineDownloading))
			// Expect(pipeline.Status.StageStatuses.DownloadStatus).To(Equal(v1beta1.PipelineStageInProgress))
			// Expect(pipeline.Status.StageStatuses.UploadStatus).To(Equal(v1beta1.PipelineStageNotStarted))
			// Expect(pipeline.Status.StageStatuses.ScanStatus).To(Equal(v1beta1.PipelineStageNotStarted))
		})

	})
	When("a pipeline conditionally uses a container", func() {
		var (
			suffix                    = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
			includeContainerName      = "should-include"
			doNotIncludeContainerName = "do-no-include"
			profile                   *v1beta1.Profile
			pipeline                  *v1beta1.Pipeline
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
								Image: testImage,
								Name:  includeContainerName,
							},
							IncludeIf: &v1beta1.ContainerCondition{
								WhenParamSet: "DEFAULT_SET",
							},
						},
						{
							Container: corev1.Container{
								Image: testImage,
								Name:  doNotIncludeContainerName,
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
					ServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
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

			scanPod := &corev1.Pod{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: pipelineResourcePrefix + pipeline.Name, Namespace: pipeline.Namespace}, scanPod)
			Expect(err).NotTo(HaveOccurred())
			ValidatePipelinePodSpec(
				scanPod.Spec,
				sidecarImage,
				pipeline,
				profile,
				downloader,
				func(c corev1.Container) {
					Expect(c.Name).To(Equal(downloadContainerPrefix + downloadContainerName))
					Expect(c.Image).To(Equal(testImage))
				},
				map[string]func(_ corev1.Container){
					includeContainerName: func(c corev1.Container) {
						Expect(c.Env).To(ContainElements(corev1.EnvVar{
							Name:  "OCULAR_PARAM_DEFAULT_EMPTY",
							Value: "",
						}, corev1.EnvVar{
							Name:  "OCULAR_PARAM_DEFAULT_SET",
							Value: "1",
						}))
					},
				},
				nil,
			)
		})

	})
})

func ValidatePipelinePodSpec(podSpec corev1.PodSpec,
	sidecarImage string,
	pipeline *v1beta1.Pipeline,
	profile *v1beta1.Profile,
	downloader *v1beta1.Downloader,
	downloaderValidator func(corev1.Container),
	scanContainerValidators map[string]func(corev1.Container),
	uploadContainerValidators map[string]func(corev1.Container),
) {
	Expect(podSpec.InitContainers).To(HaveLen(2)) // downloader + sidecar
	scanContainers, uploadContainers := make(map[string]corev1.Container), make(map[string]corev1.Container)
	for _, c := range podSpec.Containers {
		if name, found := strings.CutPrefix(c.Name, scanContainerPrefix); found {
			scanContainers[name] = c
		} else if name, found := strings.CutPrefix(c.Name, uploadContainerPrefix); found {
			uploadContainers[name] = c
		}
	}

	Expect(scanContainers).To(HaveLen(len(scanContainerValidators)))
	for n, s := range scanContainers {
		Expect(scanContainerValidators).To(HaveKey(n))
		validator := scanContainerValidators[n]
		if validator != nil {
			validator(s)
		}
	}

	Expect(uploadContainers).To(HaveLen(len(uploadContainerValidators)))
	for n, u := range uploadContainers {
		Expect(uploadContainerValidators).To(HaveKey(n))
		validator := uploadContainerValidators[n]
		if validator != nil {
			validator(u)
		}
	}

	// Sidecar
	Expect(podSpec.InitContainers[0].Name).To(Equal(sidecarInitContainerName))
	Expect(podSpec.InitContainers[0].Image).To(Equal(sidecarImage))
	Expect(podSpec.InitContainers[0].RestartPolicy).To(BeNil())
	// Downloader
	if downloaderValidator != nil {
		downloaderValidator(podSpec.InitContainers[1])
	}

	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.ServiceAccountName))
	Expect(podSpec.RuntimeClassName).To(Equal(pipeline.Spec.RuntimeClassName))
	expectedEnv := []any{
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
	}
	for _, container := range podSpec.Containers {
		Expect(container.Env).To(ContainElements(
			expectedEnv...,
		))
	}
}

// func ValidatePipelineUploadPodSpec(podSpec corev1.PodSpec,
// 	sidecarImage string,
// 	pipeline *v1beta1.Pipeline,
// 	profile *v1beta1.Profile) {
// 	Expect(podSpec.InitContainers).To(HaveLen(1)) // sidecar only
// 	Expect(podSpec.Containers).To(HaveLen(len(profile.Spec.UploaderRefs)))
// 	// sidecar
// 	Expect(podSpec.InitContainers[0].Name).To(Equal(sidecarReceiverContainerName))
// 	Expect(podSpec.InitContainers[0].Image).To(Equal(sidecarImage))
// 	Expect(podSpec.InitContainers[0].Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))

// 	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.UploadServiceAccountName))
// 	Expect(podSpec.RuntimeClassName).To(Equal(pipeline.Spec.RuntimeClassName))
// 	for _, container := range podSpec.Containers {
// 		Expect(container.Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))
// 		Expect(container.Env).To(ContainElements(
// 			corev1.EnvVar{
// 				Name:  "OCULAR_PARAM_PARAM1",
// 				Value: "value1",
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_PARAM_INHERIT",
// 				Value: "profile-invocation",
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_PARAM_INHERIT_DEFAULT",
// 				Value: "profile-default",
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_TARGET_IDENTIFIER",
// 				Value: pipeline.Spec.Target.Identifier,
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_RESULTS_DIR",
// 				Value: "/mnt/results",
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_PIPELINE_NAME",
// 				Value: pipeline.Name,
// 			},
// 			corev1.EnvVar{
// 				Name: "OCULAR_POD_NAME",
// 				ValueFrom: &corev1.EnvVarSource{
// 					FieldRef: &corev1.ObjectFieldSelector{
// 						APIVersion: "v1",
// 						FieldPath:  "metadata.name",
// 					},
// 				},
// 			},
// 			corev1.EnvVar{
// 				Name:  "OCULAR_TARGET_VERSION",
// 				Value: pipeline.Spec.Target.Version,
// 			},
// 		))
// 	}

// }
