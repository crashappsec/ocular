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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Pipeline Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName   = "test-resource"
			downloaderName = "test-downloader"
			profileName    = "test-profile"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		pipeline := &ocularcrashoverriderunv1beta1.Pipeline{}

		downloaderTypeNamespacedName := types.NamespacedName{
			Name:      downloaderName,
			Namespace: "default",
		}
		downloader := &ocularcrashoverriderunv1beta1.Downloader{}

		profileTypeNamespacedName := types.NamespacedName{
			Name:      profileName,
			Namespace: "default",
		}
		profile := &ocularcrashoverriderunv1beta1.Profile{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Pipeline")

			err := k8sClient.Get(ctx, downloaderTypeNamespacedName, downloader)
			if err != nil && errors.IsNotFound(err) {
				downloaderResource := &ocularcrashoverriderunv1beta1.Downloader{
					TypeMeta: metav1.TypeMeta{
						Kind: "Downloader",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      downloaderName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1beta1.DownloaderSpec{
						Container: corev1.Container{
							Name:    "downloader-container",
							Image:   "alpine:latest",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo Downloading...; echo $OCULAR_TARGET_IDENTIFIER > ./target.txt"},
						},
					},
					Status: ocularcrashoverriderunv1beta1.DownloaderStatus{
						Valid: ptr.To(true),
					},
				}
				Expect(k8sClient.Create(ctx, downloaderResource)).To(Succeed())
				downloaderResource.Status.Valid = ptr.To(true)
				Expect(k8sClient.Status().Update(ctx, downloaderResource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, profileTypeNamespacedName, profile)
			if err != nil && errors.IsNotFound(err) {
				profileResource := &ocularcrashoverriderunv1beta1.Profile{
					TypeMeta: metav1.TypeMeta{
						Kind: "Profile",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      profileName,
						Namespace: "default",
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
					Status: ocularcrashoverriderunv1beta1.ProfileStatus{
						Valid: ptr.To(true),
					},
				}
				Expect(k8sClient.Create(ctx, profileResource)).To(Succeed())
				profileResource.Status.Valid = ptr.To(true)
				Expect(k8sClient.Status().Update(ctx, profileResource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedName, pipeline)
			if err != nil && errors.IsNotFound(err) {
				resource := &ocularcrashoverriderunv1beta1.Pipeline{
					TypeMeta: metav1.TypeMeta{
						Kind: "Pipeline",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1beta1.PipelineSpec{
						DownloaderRef: downloaderName,
						ProfileRef:    profileName,
						Target: ocularcrashoverriderunv1beta1.Target{
							Identifier: "https://example.com/samplefile.txt",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			downloaderResource := &ocularcrashoverriderunv1beta1.Downloader{}
			err := k8sClient.Get(ctx, downloaderTypeNamespacedName, downloaderResource)
			Expect(err).NotTo(HaveOccurred())

			profileResource := &ocularcrashoverriderunv1beta1.Profile{}
			err = k8sClient.Get(ctx, profileTypeNamespacedName, profileResource)
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1beta1.Pipeline{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Profile")
			Expect(k8sClient.Delete(ctx, profileResource)).To(Succeed())
			By("Cleanup the specific resource instance Downloader")
			Expect(k8sClient.Delete(ctx, downloaderResource)).To(Succeed())
			By("Cleanup the specific resource instance Pipeline")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PipelineReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				ExtractorImage: "ocular-controller:latest",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1beta1.Pipeline{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.ScanJobOnly).To(BeTrue())

			Expect(resource.Status.ScanJob).ToNot(BeNil())
			Expect(resource.Status.ScanJob.Namespace).To(Equal(resource.Namespace))
			scanJob := &batchv1.Job{}
			scanJobName := types.NamespacedName{
				Name:      resource.Status.ScanJob.Name,
				Namespace: resource.Status.ScanJob.Namespace,
			}
			err = k8sClient.Get(ctx, scanJobName, scanJob)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
