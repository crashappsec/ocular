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

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Crawler Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		crawler := &ocularcrashoverriderunv1beta1.Crawler{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Crawler")
			err := k8sClient.Get(ctx, typeNamespacedName, crawler)
			if err != nil && errors.IsNotFound(err) {
				resource := &ocularcrashoverriderunv1beta1.Crawler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1beta1.CrawlerSpec{
						Container: corev1.Container{
							Image:   "alpine:latest",
							Name:    "uploader-container",
							Env:     []corev1.EnvVar{},
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo hello world"},
						},
						Parameters: map[string]ocularcrashoverriderunv1beta1.ParameterDefinition{
							"MY_PARAM": {
								Description: "A sample parameter",
								Required:    true,
							},
							"OPTIONAL_PARAM": {
								Description: "An optional parameter",
								Required:    false,
								Default:     "default_value",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &ocularcrashoverriderunv1beta1.Crawler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Crawler")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &CrawlerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1beta1.Crawler{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Valid).To(Not(BeNil()))
			Expect(*resource.Status.Valid).To(BeTrue())
		})
	})
})
