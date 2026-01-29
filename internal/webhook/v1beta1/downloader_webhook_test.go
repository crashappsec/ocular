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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Downloader Webhook", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace  = "default"
		obj        *ocularcrashoverriderunv1beta1.Downloader
		oldObj     *ocularcrashoverriderunv1beta1.Downloader
		pipeline   *ocularcrashoverriderunv1beta1.Pipeline
		profile    *ocularcrashoverriderunv1beta1.Profile
		svcAccount *corev1.ServiceAccount
		validator  DownloaderCustomValidator
	)

	BeforeEach(func() {
		profile = &ocularcrashoverriderunv1beta1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "downloader-webhook-test-profile",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.ProfileSpec{
				Containers: []corev1.Container{
					testutils.GenerateRandomContainer(rnd),
				},
			},
		}
		obj = &ocularcrashoverriderunv1beta1.Downloader{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "downloader-webhook-test",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.DownloaderSpec{
				Container: testutils.GenerateRandomContainer(rnd),
			},
		}
		svcAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespace,
			},
		}
		oldObj = &ocularcrashoverriderunv1beta1.Downloader{}
		pipeline = &ocularcrashoverriderunv1beta1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "downloader-webhook-test-pipeline",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
				ProfileRef: corev1.ObjectReference{
					Name: profile.Name,
				},
				DownloaderRef: corev1.ObjectReference{
					Name: obj.Name,
				},
				Target: ocularcrashoverriderunv1beta1.Target{
					Identifier: "some-identifier",
					Version:    "some-version",
				},
				ScanServiceAccountName:   svcAccount.Name,
				UploadServiceAccountName: svcAccount.Name,
			},
		}
		validator = DownloaderCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		Expect(profile).NotTo(BeNil(), "Expected profile to be initialized")
		Expect(pipeline).NotTo(BeNil(), "Expected pipeline to be initialized")
	})

	AfterEach(func() {
		var err error
		if pipeline != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, pipeline))
			Expect(err).ToNot(HaveOccurred())
		}
		if profile != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, profile))
			Expect(err).ToNot(HaveOccurred())
		}
		if obj != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, obj))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When deleting a Downloader under Validating Webhook", func() {
		It("Should deny deletion if referenced by a Pipeline", func() {
			By("Creating a profile for the pipeline")
			Expect(k8sClient.Create(ctx, profile)).Should(Succeed())
			By("creating a service account for the pipeline")
			Expect(k8sClient.Create(ctx, svcAccount)).Should(Succeed())
			By("Creating the downloader for the pipeline")
			Expect(k8sClient.Create(ctx, obj)).Should(Succeed())
			By("Creating a pipeline that references the downloader")
			Expect(k8sClient.Create(ctx, pipeline)).Should(Succeed())
			By("simulating a deletion scenario")
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})

		It("Should allow deletion if not referenced by any Pipeline", func() {
			By("simulating a deletion scenario")
			Expect(validator.ValidateDelete(ctx, obj)).Error().ToNot(HaveOccurred())
		})
	})

})
