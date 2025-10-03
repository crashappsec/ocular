// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	testutils "github.com/crashappsec/ocular/test/utils"
)

var _ = Describe("Pipeline Controller", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace      = "default"
		extractorImage = testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet) + ":latest"
		downloader     = &ocularcrashoverriderunv1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-downloader",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.DownloaderSpec{
				Container: corev1.Container{
					Name:    "downloader-container",
					Image:   "alpine:latest",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"echo Downloading...; echo $OCULAR_TARGET_IDENTIFIER > ./target.txt"},
				},
			},
		}
	)
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, downloader)).To(Succeed())
		downloader.ObjectMeta = metav1.ObjectMeta{
			Name:      "test-downloader",
			Namespace: namespace,
		}
	})

	When("a pipeline uses a profile with no uploaders", func() {
		var (
			profile = &ocularcrashoverriderunv1beta1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: namespace,
				},
				Spec: ocularcrashoverriderunv1beta1.ProfileSpec{
					Containers: []corev1.Container{
						{
							Image:   "alpine:latest",
							Name:    "profile-container",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
						},
					},
					Artifacts: []string{"results.txt"},
				},
			}
			pipeline = &ocularcrashoverriderunv1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: namespace,
				},
				Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
					DownloaderRef: corev1.ObjectReference{
						Name: downloader.Name,
					},
					ProfileRef: corev1.ObjectReference{
						Name:      profile.Name,
						Namespace: namespace,
					},
					Target: ocularcrashoverriderunv1beta1.Target{
						Identifier: "https://example.com/samplefile.txt",
					},
					ScanServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
				},
			}
		)

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			Expect(k8sClient.Create(ctx, pipeline)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, pipeline)
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
		})

		It("should create the pipeline", func() {
			By("Creating only a pod for the scanners")
			controllerReconciler := &PipelineReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				ExtractorImage: extractorImage,
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			updatedResource := &ocularcrashoverriderunv1beta1.Pipeline{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, updatedResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedResource.Status.ScanPodOnly).To(BeTrue())

			scanPods := &corev1.PodList{}
			err = k8sClient.List(ctx, scanPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					TypeLabelKey:     PodTypeScan,
					PipelineLabelKey: pipeline.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(scanPods.Items).To(HaveLen(1))
			ValidatePipelineScanPodSpec(scanPods.Items[0].Spec, extractorImage, pipeline, profile, downloader)

			uploadPods := &corev1.PodList{}
			err = k8sClient.List(ctx, scanPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					TypeLabelKey:     PodTypeUpload,
					PipelineLabelKey: pipeline.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(uploadPods.Items).To(BeEmpty())
		})
	})

	When("a pipeline uses a profile with at least one uploader", func() {
		var (
			suffix   = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
			uploader = &ocularcrashoverriderunv1beta1.Uploader{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-uploader-" + suffix,
					Namespace: namespace,
				},
				Spec: ocularcrashoverriderunv1beta1.UploaderSpec{
					Container: corev1.Container{
						Name:    "uploader-container",
						Image:   "alpine:latest",
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{"echo uploading...; cat $OCULAR_RESULTS_DIR/results.txt; echo done."},
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterDefinition{
						{
							Name:        "PARAM1",
							Description: "A sample parameter for the uploader",
							Required:    true,
						},
					},
				},
			}
			profile = &ocularcrashoverriderunv1beta1.Profile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile-" + suffix,
					Namespace: namespace,
				},
				Spec: ocularcrashoverriderunv1beta1.ProfileSpec{
					Containers: []corev1.Container{
						{
							Image:   "alpine:latest",
							Name:    "profile-container",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo scanning...; sha256sum $(cat ./target.txt) > $OCULAR_RESULTS_DIR/results.txt"},
						},
					},
					Artifacts: []string{"results.txt"},
					UploaderRefs: []ocularcrashoverriderunv1beta1.UploaderObjectReference{
						{ObjectReference: corev1.ObjectReference{
							Name:      uploader.Name,
							Namespace: uploader.Namespace,
						},
							Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
								{
									Name:  "PARAM1",
									Value: "value1",
								},
							},
						},
					},
				},
			}
			pipeline = &ocularcrashoverriderunv1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-" + suffix,
					Namespace: namespace,
				},
				Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
					DownloaderRef: corev1.ObjectReference{
						Name: downloader.Name,
					},
					ProfileRef: corev1.ObjectReference{
						Name:      profile.Name,
						Namespace: namespace,
					},
					Target: ocularcrashoverriderunv1beta1.Target{
						Identifier: "https://example.com/samplefile.txt",
					},
					ScanServiceAccountName:   testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
					UploadServiceAccountName: testutils.GenerateRandomString(rnd, 10, testutils.LowercaseAlphabeticLetterSet),
				},
			}
		)

		BeforeEach(func() {
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
			By("creating only a scanner job and a uploader job")
			controllerReconciler := &PipelineReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				ExtractorImage: extractorImage,
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pipeline.Name,
					Namespace: pipeline.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			updatedResource := &ocularcrashoverriderunv1beta1.Pipeline{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      pipeline.Name,
				Namespace: pipeline.Namespace,
			}, updatedResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedResource.Status.ScanPodOnly).To(BeFalse())

			scanPods := &corev1.PodList{}
			err = k8sClient.List(ctx, scanPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					TypeLabelKey:     PodTypeScan,
					PipelineLabelKey: pipeline.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(scanPods.Items).To(HaveLen(1))
			ValidatePipelineScanPodSpec(scanPods.Items[0].Spec, extractorImage, pipeline, profile, downloader)

			uploadPods := &corev1.PodList{}
			err = k8sClient.List(ctx, uploadPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					TypeLabelKey:     PodTypeUpload,
					PipelineLabelKey: pipeline.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(uploadPods.Items).To(HaveLen(1))
			ValidatePipelineUploadPodSpec(uploadPods.Items[0].Spec, extractorImage, pipeline, profile)
		})
	})
})

func ValidatePipelineScanPodSpec(podSpec corev1.PodSpec,
	extractorImage string,
	pipeline *ocularcrashoverriderunv1beta1.Pipeline,
	profile *ocularcrashoverriderunv1beta1.Profile,
	downloader *ocularcrashoverriderunv1beta1.Downloader) {
	Expect(podSpec.InitContainers).To(HaveLen(2)) // downloader + extractor
	Expect(podSpec.Containers).To(HaveLen(len(profile.Spec.Containers)))
	// Downloader
	Expect(podSpec.InitContainers[0].Name).To(Equal(downloader.Spec.Container.Name))
	Expect(podSpec.InitContainers[0].Image).To(Equal(downloader.Spec.Container.Image))
	Expect(podSpec.InitContainers[0].Command).To(Equal(downloader.Spec.Container.Command))
	Expect(podSpec.InitContainers[0].Args).To(Equal(downloader.Spec.Container.Args))

	// Extractor
	Expect(podSpec.InitContainers[1].Name).To(Equal("extract-artifacts"))
	Expect(podSpec.InitContainers[1].Image).To(Equal(extractorImage))
	Expect(podSpec.InitContainers[1].Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))
	Expect(podSpec.InitContainers[1].RestartPolicy).ToNot(BeNil())
	Expect(*podSpec.InitContainers[1].RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways)) // is a sidecar

	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.ScanServiceAccountName))
	for _, container := range podSpec.Containers {
		Expect(container.Env).To(ContainElements(
			corev1.EnvVar{
				Name:  "OCULAR_TARGET_IDENTIFIER",
				Value: pipeline.Spec.Target.Identifier,
			},
			corev1.EnvVar{
				Name:  "OCULAR_RESULTS_DIR",
				Value: "/mnt/results",
			},
			corev1.EnvVar{
				Name: "OCULAR_PIPELINE_NAME",
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
		))
	}
}

func ValidatePipelineUploadPodSpec(podSpec corev1.PodSpec,
	extractorImage string,
	pipeline *ocularcrashoverriderunv1beta1.Pipeline,
	profile *ocularcrashoverriderunv1beta1.Profile) {
	Expect(podSpec.InitContainers).To(HaveLen(1)) // extractor only
	Expect(podSpec.Containers).To(HaveLen(len(profile.Spec.UploaderRefs)))
	// Extractor
	Expect(podSpec.InitContainers[0].Name).To(Equal("receive-artifacts"))
	Expect(podSpec.InitContainers[0].Image).To(Equal(extractorImage))
	Expect(podSpec.InitContainers[0].Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))

	Expect(podSpec.ServiceAccountName).To(Equal(pipeline.Spec.UploadServiceAccountName))
	for _, container := range podSpec.Containers {
		Expect(container.Args).Should(HaveLen(len(profile.Spec.Artifacts) + 2))
		Expect(container.Env).To(ContainElements(
			corev1.EnvVar{
				Name:  "OCULAR_TARGET_IDENTIFIER",
				Value: pipeline.Spec.Target.Identifier,
			},
			corev1.EnvVar{
				Name:  "OCULAR_RESULTS_DIR",
				Value: "/mnt/results",
			},
			corev1.EnvVar{
				Name: "OCULAR_PIPELINE_NAME",
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
