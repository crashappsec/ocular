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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Search Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			crawlerName  = "test-crawler"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		search := &ocularcrashoverriderunv1beta1.Search{}

		crawlerTypeNamespacedName := types.NamespacedName{
			Name:      crawlerName,
			Namespace: "default",
		}
		crawler := &ocularcrashoverriderunv1beta1.Crawler{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Search")

			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawler)
			if err != nil && errors.IsNotFound(err) {
				crawlerResource := &ocularcrashoverriderunv1beta1.Crawler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      crawlerName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1beta1.CrawlerSpec{
						Container: corev1.Container{
							Name:    "crawler-container",
							Image:   "alpine:latest",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo crawling $CRAWL_TARGET ...; sleep 10; echo done."},
						},
						Parameters: []ocularcrashoverriderunv1beta1.ParameterDefinition{
							{
								Name:        "CRAWL_TARGET",
								Description: "The search query to execute",
								Required:    true,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, crawlerResource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedName, search)
			if err != nil && errors.IsNotFound(err) {
				resource := &ocularcrashoverriderunv1beta1.Search{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: ocularcrashoverriderunv1beta1.SearchSpec{
						CrawlerRef: ocularcrashoverriderunv1beta1.CrawlerObjectReference{
							ObjectReference: corev1.ObjectReference{
								Name: crawlerName,
							},
							Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
								{
									Name:  "CRAWL_TARGET",
									Value: "example search query",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {

			crawlerResource := &ocularcrashoverriderunv1beta1.Crawler{}
			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawlerResource)
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1beta1.Search{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Search")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, crawlerResource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SearchReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SearchClusterRole: "test-search-cluster-role",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &ocularcrashoverriderunv1beta1.Search{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.StartTime).ToNot(BeNil())

			searchPods := &corev1.PodList{}
			err = k8sClient.List(ctx, searchPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					TypeLabelKey:   PodTypeSearch,
					SearchLabelKey: resource.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(searchPods.Items).To(HaveLen(1))

			searchRB := &rbacv1.RoleBinding{}
			searchRBName := types.NamespacedName{
				Name:      resource.GetName() + searchResourceSuffix,
				Namespace: resource.GetNamespace(),
			}
			err = k8sClient.Get(ctx, searchRBName, searchRB)
			Expect(err).NotTo(HaveOccurred())

		})
	})
})
