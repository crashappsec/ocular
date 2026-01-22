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

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	testutils "github.com/crashappsec/ocular/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Pipeline Webhook", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace  = "default"
		obj        *ocularcrashoverriderunv1beta1.Pipeline
		oldObj     *ocularcrashoverriderunv1beta1.Pipeline
		profile    *ocularcrashoverriderunv1beta1.Profile
		downloader *ocularcrashoverriderunv1beta1.Downloader
		svcAccount *corev1.ServiceAccount
		validator  PipelineCustomValidator
		defaulter  PipelineCustomDefaulter
	)

	BeforeEach(func() {
		profile = &ocularcrashoverriderunv1beta1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline-webhook-test-profile",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.ProfileSpec{
				Containers: []corev1.Container{
					testutils.GenerateRandomContainer(rnd),
					testutils.GenerateRandomContainer(rnd),
				},
			},
		}
		svcAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespace,
			},
		}
		downloader = &ocularcrashoverriderunv1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline-webhook-test-downloader",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.DownloaderSpec{
				Container: testutils.GenerateRandomContainer(rnd),
			},
		}
		obj = &ocularcrashoverriderunv1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline-webhook-test",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
				ProfileRef: corev1.ObjectReference{
					Name:      profile.Name,
					Namespace: profile.Namespace,
				},
				DownloaderRef: corev1.ObjectReference{
					Name:      downloader.Name,
					Namespace: downloader.Namespace,
				},
				ScanServiceAccountName:   svcAccount.Name,
				UploadServiceAccountName: svcAccount.Name,
			},
		}
		oldObj = &ocularcrashoverriderunv1beta1.Pipeline{}
		validator = PipelineCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = PipelineCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		Expect(profile).NotTo(BeNil(), "Expected profile to be initialized")
		Expect(downloader).NotTo(BeNil(), "Expected downloader to be initialized")
	})

	JustAfterEach(func() {
		var err error
		if profile != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, profile))
			Expect(err).ToNot(HaveOccurred())
		}
		if downloader != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, downloader))
			Expect(err).ToNot(HaveOccurred())
		}
		if svcAccount != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, svcAccount))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When creating Pipeline under Defaulting Webhook", func() {
		It("Should set the service account fields to default", func() {
			By("simulating a scenario where defaults should be applied")
			obj.Spec.ScanServiceAccountName = "some-account"
			obj.Spec.UploadServiceAccountName = ""
			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			By("checking that the default values are set")
			Expect(obj.Spec.ScanServiceAccountName).To(Equal("some-account"))
			Expect(obj.Spec.UploadServiceAccountName).To(Equal("default"))
		})
	})

	Context("when creating a pipeline with a validating webhook", func() {
		It("Should deny creation if downloader is not found", func() {
			By("only creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if profile is not found", func() {
			By("only creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if service account is not found", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if the name is too long", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			obj.Name = "this-name-is-way-too-long-and-should-fail-validation-because-it-is-way-too-long-2"
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(
				MatchError(ContainSubstring("must be no more than 52 characters")),
				"Expected name validation to fail for a too-long name")
		})

		It("Should deny creation if downloader and profile volumes container duplicate names", func() {
			By("creating the profile")
			profile.Spec.Volumes = []corev1.Volume{
				{Name: "duplicate-volume-name"},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			downloader.Spec.Volumes = []corev1.Volume{
				{Name: "duplicate-volume-name"},
			}
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		})

		It("Should admit creation if downloader, profile and service account exist", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).To(BeNil())
		})
	})

	Context("when updating a pipeline with a validating webhook", func() {
		It("Should deny creation if downloader is not found", func() {
			By("only creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if profile is not found", func() {
			By("only creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if service account is not found", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("Should deny creation if the name is too long", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			obj.Name = "this-name-is-way-too-long-and-should-fail-validation-because-it-is-way-too-long"
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(
				MatchError(ContainSubstring("must be no more than 52 characters")),
				"Expected name validation to fail for a too-long name")
		})

		It("Should deny creation if downloader and profile volumes container duplicate names", func() {
			By("creating the profile")
			profile.Spec.Volumes = []corev1.Volume{
				{Name: "duplicate-volume-name"},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			downloader.Spec.Volumes = []corev1.Volume{
				{Name: "duplicate-volume-name"},
			}
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred())
		})

		It("Should admit creation if downloader, profile and service account exist", func() {
			By("creating the profile")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			By("creating the downloader")
			Expect(k8sClient.Create(ctx, downloader)).To(Succeed())
			By("creating the default service account")
			Expect(k8sClient.Create(ctx, svcAccount)).To(Succeed())
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).To(BeNil())
		})
	})

})
