// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	"math/rand"

	testutils "github.com/crashappsec/ocular/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Profile Webhook", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace         = "default"
		obj               *ocularcrashoverriderunv1beta1.Profile
		oldObj            *ocularcrashoverriderunv1beta1.Profile
		validator         ProfileCustomValidator
		uploader1         *ocularcrashoverriderunv1beta1.Uploader
		uploader2         *ocularcrashoverriderunv1beta1.Uploader
		pipeline          *ocularcrashoverriderunv1beta1.Pipeline
		downloader        *ocularcrashoverriderunv1beta1.Downloader
		defaultSVCAccount *v1.ServiceAccount
	)

	BeforeEach(func() {
		obj = &ocularcrashoverriderunv1beta1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-webhook-test",
				Namespace: namespace,
			},
		}
		oldObj = &ocularcrashoverriderunv1beta1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-webhook-test",
				Namespace: namespace,
			},
		}
		uploader1 = &ocularcrashoverriderunv1beta1.Uploader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uploader1",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.UploaderSpec{
				Container: testutils.GenerateRandomContainer(rnd),
				Parameters: []ocularcrashoverriderunv1beta1.ParameterDefinition{
					{
						Name:     "UPLOADER_1_PARAM_1",
						Required: true,
					},
					{
						Name:     "UPLOADER_1_PARAM_2",
						Default:  ptr.To("uploader 1 param 2"),
						Required: false,
					},
				},
			},
		}
		uploader2 = &ocularcrashoverriderunv1beta1.Uploader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uploader2",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.UploaderSpec{
				Container: testutils.GenerateRandomContainer(rnd),
				Parameters: []ocularcrashoverriderunv1beta1.ParameterDefinition{
					{
						Name:     "UPLOADER_2_PARAM_1",
						Required: false,
					},
					{
						Name:     "UPLOADER_2_PARAM_2",
						Required: true,
					},
				},
			},
		}
		downloader = &ocularcrashoverriderunv1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-downloader",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.DownloaderSpec{
				Container: testutils.GenerateRandomContainer(rnd),
			},
		}
		defaultSVCAccount = &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespace,
			},
		}
		pipeline = &ocularcrashoverriderunv1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline-webhook-test",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
				ProfileRef: v1.ObjectReference{
					Name:      obj.Name,
					Namespace: obj.Namespace,
				},
				DownloaderRef: ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
					ObjectReference: v1.ObjectReference{
						Name:      downloader.Name,
						Namespace: namespace,
					},
				},
				Target: ocularcrashoverriderunv1beta1.Target{
					Identifier: "some-target",
					Version:    "v1.2.3",
				},
			},
		}
		validator = ProfileCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		Expect(uploader1).NotTo(BeNil(), "Expected uploader1 to be initialized")
		Expect(uploader2).NotTo(BeNil(), "Expected uploader2 to be initialized")
		Expect(downloader).NotTo(BeNil(), "Expected downloader to be initialized")
		Expect(defaultSVCAccount).NotTo(BeNil(), "Expected defaultSVCAccount to be initialized")
		Expect(pipeline).NotTo(BeNil(), "Expected pipeline to be initialized")
	})

	AfterEach(func() {
		var err error
		if pipeline != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, pipeline))
			Expect(err).ToNot(HaveOccurred())
		}
		if uploader1 != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, uploader1))
			Expect(err).ToNot(HaveOccurred())
		}
		if uploader2 != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, uploader2))
			Expect(err).ToNot(HaveOccurred())
		}
		if downloader != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, downloader))
			Expect(err).ToNot(HaveOccurred())
		}
		if defaultSVCAccount != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, defaultSVCAccount))
			Expect(err).ToNot(HaveOccurred())
		}
		if obj != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, obj))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When creating a new profile under validating webhook", func() {
		It("should succeed if no uploaders are referenced", func() {
			By("not referencing any uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(Succeed())
		})

		It("should fail if referenced uploaders do not exist", func() {
			By("setting uploader references to non-existent uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      "non-existent-uploader-1",
						Namespace: namespace,
					},
				}}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("should fail if uploader reference is invalid", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			By("not defining a required parameter for a referenced uploader")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_1_PARAM_2",
							Value: "optional parameter set",
						},
					},
				}}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("should succeed if all referenced uploaders are valid", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			Expect(k8sClient.Create(ctx, uploader2)).To(Succeed())
			By("defining all required parameters for referenced uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_1_PARAM_1",
							Value: "required parameter set",
						},
					},
				},
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader2.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_2_PARAM_2",
							Value: "required parameter set",
						},
					},
				},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(Succeed())
		})

		It("should fail if a referenced uploader is in a different namespace", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			By("setting the namespace of a referenced uploader to a different namespace than the profile")
			obj.Namespace = "different-namespace-1"
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
				},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("should fail if two referenced uploaders define the same volume name", func() {
			By("defining the same volume name in two referenced uploaders")
			uploader1.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{}
			uploader1.Spec.Volumes = []v1.Volume{
				{Name: "shared-volume", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
			}
			uploader2.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{}
			uploader2.Spec.Volumes = []v1.Volume{
				{Name: "shared-volume", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
			}
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			Expect(k8sClient.Create(ctx, uploader2)).To(Succeed())
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
				},
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader2.Name,
						Namespace: namespace,
					},
				},
			}
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred(), "Expected validation to fail due to duplicate volume names across referenced uploaders")
		})
	})

	Context("When updating a Profile under Validating Webhook", func() {
		It("should succeed if no uploaders are referenced", func() {
			By("not referencing any uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(Succeed())
		})

		It("should fail if referenced uploaders do not exist", func() {
			By("setting uploader references to non-existent uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      "non-existent-uploader-1",
						Namespace: namespace,
					},
				}}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("should fail if uploader reference is invalid", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			By("not defining a required parameter for a referenced uploader")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_1_PARAM_2",
							Value: "optional parameter set",
						},
					},
				}}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("should succeed if all referenced uploaders are valid", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			Expect(k8sClient.Create(ctx, uploader2)).To(Succeed())
			By("defining all required parameters for referenced uploaders")
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_1_PARAM_1",
							Value: "required parameter set",
						},
					},
				},
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader2.Name,
						Namespace: namespace,
					},
					Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
						{
							Name:  "UPLOADER_2_PARAM_2",
							Value: "required parameter set",
						},
					},
				},
			}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(Succeed())
		})

		It("should fail if a referenced uploader is in a different namespace", func() {
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			By("setting the namespace of a referenced uploader to a different namespace than the profile")
			obj.Namespace = "different-namespace"
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
				},
			}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("should fail if two referenced uploaders define the same volume name", func() {
			By("defining the same volume name in two referenced uploaders")
			uploader1.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{}
			uploader1.Spec.Volumes = []v1.Volume{
				{Name: "shared-volume", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
			}
			uploader2.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{}
			uploader2.Spec.Volumes = []v1.Volume{
				{Name: "shared-volume", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
			}
			Expect(k8sClient.Create(ctx, uploader1)).To(Succeed())
			Expect(k8sClient.Create(ctx, uploader2)).To(Succeed())
			obj.Spec.UploaderRefs = []ocularcrashoverriderunv1beta1.ParameterizedObjectReference{
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader1.Name,
						Namespace: namespace,
					},
				},
				{
					ObjectReference: v1.ObjectReference{
						Name:      uploader2.Name,
						Namespace: namespace,
					},
				},
			}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred(), "Expected validation to fail due to duplicate volume names across referenced uploaders")
		})
	})

	Context("When deleting a Profile under Validating Webhook", func() {
		It("should succeed if no pipelines reference the profile", func() {
			By("not having any pipelines reference the profile")
			Expect(validator.ValidateDelete(ctx, obj)).Error().To(Succeed())
		})

		It("should fail if a pipeline references the profile", func() {
			By("creating a profile")
			obj.Spec.Containers = []v1.Container{testutils.GenerateRandomContainer(rnd)} // profiles must have at least one container
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			By("creating a downloader the pipeline can reference")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account the pipeline will use")
			Expect(k8sClient.Create(ctx, defaultSVCAccount)).To(Succeed())
			By("creating a pipeline that references the profile, then attempting to delete the profile")
			Expect(k8sClient.Create(ctx, pipeline)).To(Succeed())

			_, err := validator.ValidateDelete(ctx, obj)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})
	})

})
