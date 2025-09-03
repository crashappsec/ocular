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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocularcrashoverriderunv1 "github.com/crashappsec/ocular/api/v1"
)

var _ = Describe("Downloader Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		downloader := &ocularcrashoverriderunv1.Downloader{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Downloader")
			err := k8sClient.Get(ctx, typeNamespacedName, downloader)
			if err != nil && errors.IsNotFound(err) {
				resource := &ocularcrashoverriderunv1.Downloader{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Downloader",
						APIVersion: "ocular.crashoverride.run/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1.DownloaderSpec{
						Container: corev1.Container{
							Image:   "alpine:latest",
							Name:    "downloader-container",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo Downloading...; wget -O ./data.txt $OCULAR_TARGET_IDENTIFIER"},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &ocularcrashoverriderunv1.Downloader{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Downloader")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DownloaderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1.Downloader{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Finalizers).To(ContainElements("downloader.finalizers.ocular.crashoverride.run/cleanup"))

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Valid).To(BeTrue())
		})
	})
})
