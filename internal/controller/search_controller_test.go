// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Search Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName         = "test-resource"
			crawlerName          = "test-crawler"
			crawlerContainerName = "crawler-container"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		search := &v1beta1.Search{}

		crawlerTypeNamespacedName := types.NamespacedName{
			Name:      crawlerName,
			Namespace: "default",
		}
		crawler := &v1beta1.Crawler{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Search")

			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawler)
			if err != nil && errors.IsNotFound(err) {
				crawlerResource := &v1beta1.Crawler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      crawlerName,
						Namespace: "default",
					},
					Spec: v1beta1.CrawlerSpec{
						Container: corev1.Container{
							Name:    crawlerContainerName,
							Image:   "alpine:latest",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo crawling $CRAWL_TARGET ...; sleep 10; echo done."},
						},
						Parameters: []v1beta1.ParameterDefinition{
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
				resource := &v1beta1.Search{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1beta1.SearchSpec{
						CrawlerRef: v1beta1.ParameterizedObjectReference{
							ObjectReference: corev1.ObjectReference{
								Name: crawlerName,
							},
							Parameters: []v1beta1.ParameterSetting{
								{
									Name:  "CRAWL_TARGET",
									Value: "example search query",
								},
							},
						},
						TTLSecondsAfterFinished: ptr.To(int32(3600)),
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {

			crawlerResource := &v1beta1.Crawler{}
			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawlerResource)
			Expect(err).NotTo(HaveOccurred())

			resource := &v1beta1.Search{}
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
				SidecarImage:      "ocular-sidecar:test",
				SidecarPullPolicy: corev1.PullNever,
			}

			// First run will create the ServiceAccount since one is not specified
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &v1beta1.Search{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Spec.ServiceAccountNameOverride).ToNot(BeEmpty())

			// Second run will create the Pod and RoleBinding
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.StartTime).ToNot(BeNil())

			searchPods := &corev1.PodList{}
			err = k8sClient.List(ctx, searchPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					v1beta1.TypeLabelKey:   v1beta1.PodTypeSearch,
					v1beta1.SearchLabelKey: resource.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(searchPods.Items).To(HaveLen(1))
			searchPod := searchPods.Items[0]
			// check init containers
			Expect(searchPod.Spec.InitContainers).To(HaveLen(3)) // sidecar, sidecar-init and crawler
			Expect(searchPod.Spec.InitContainers[0].Name).To(Equal(sidecarSchedulerContainerName))
			Expect(searchPod.Spec.InitContainers[1].Name).To(Equal(sidecarInitContainerName))
			Expect(searchPod.Spec.InitContainers[2].Name).To(Equal(crawlerContainerName))
			// check containers
			Expect(searchPod.Spec.Containers).To(HaveLen(1)) // sidecar-keepalive
			Expect(searchPod.Spec.Containers[0].Name).To(Equal(sidecarKeepaliveContainerName))

			// check annotations
			templateJSON, err := json.Marshal(resource.Spec.Scheduler.PipelineTemplate)
			Expect(err).NotTo(HaveOccurred())
			Expect(searchPod.Annotations).
				To(HaveKeyWithValue(v1beta1.PipelineTemplateAnnotation, string(templateJSON)))
			ttlStr := strconv.Itoa(int(*resource.Spec.TTLSecondsAfterFinished))
			Expect(searchPod.Annotations).
				To(HaveKeyWithValue(v1beta1.TTLSecondsAnnotation, ttlStr))

			searchRB := &rbacv1.RoleBinding{}
			searchRBName := types.NamespacedName{
				Name:      resource.GetName() + searchResourceSuffix,
				Namespace: resource.GetNamespace(),
			}
			err = k8sClient.Get(ctx, searchRBName, searchRB)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("When searches schedules sub-resources", func() {
		const (
			resourceName      = "scheduler"
			crawlerName       = "test-crawler"
			childPipelineName = "child-pipeline"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		search := &v1beta1.Search{}

		crawlerTypeNamespacedName := types.NamespacedName{
			Name:      crawlerName,
			Namespace: "default",
		}
		crawler := &v1beta1.Crawler{}
		pipelineTypeNamespacedName := types.NamespacedName{
			Name:      childPipelineName,
			Namespace: "default",
		}
		pipeline := &v1beta1.Pipeline{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Search")

			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawler)
			if err != nil && errors.IsNotFound(err) {
				err = nil
				crawlerResource := &v1beta1.Crawler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      crawlerName,
						Namespace: "default",
					},
					Spec: v1beta1.CrawlerSpec{
						Container: corev1.Container{
							Name:    "crawler",
							Image:   "alpine:latest",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"echo crawling $CRAWL_TARGET ...; sleep 10; echo done."},
						}},
				}
				Expect(k8sClient.Create(ctx, crawlerResource)).To(Succeed())
			}
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, search)
			if err != nil && errors.IsNotFound(err) {
				err = nil
				resource := &v1beta1.Search{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1beta1.SearchSpec{
						CrawlerRef: v1beta1.ParameterizedObjectReference{
							ObjectReference: corev1.ObjectReference{
								Name: crawlerName,
							},
						},
						TTLSecondsAfterFinished: ptr.To(int32(3600)),
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				Expect(k8sClient.Get(ctx, typeNamespacedName, search)).To(Succeed())
			}
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Get(ctx, pipelineTypeNamespacedName, pipeline)
			if err != nil && errors.IsNotFound(err) {
				err = nil
				pipelineResource := &v1beta1.Pipeline{
					ObjectMeta: metav1.ObjectMeta{
						Name:      childPipelineName,
						Namespace: "default",
						Labels: map[string]string{
							v1beta1.ScheduledByLabelKey: search.Name,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: v1beta1.GroupVersion.String(),
								Kind:       "Search",
								Name:       search.Name,
								UID:        search.UID,
							},
						},
					},
					Spec: v1beta1.PipelineSpec{
						DownloaderRef: v1beta1.ParameterizedObjectReference{
							ObjectReference: corev1.ObjectReference{
								Name: "test",
							},
						},
						ProfileRef: corev1.ObjectReference{
							Name: "test",
						},
						Target: v1beta1.Target{
							Identifier: "test",
							Version:    "test",
						},
					},
				}
				Expect(k8sClient.Create(ctx, pipelineResource)).To(Succeed())
				Expect(k8sClient.Get(ctx, pipelineTypeNamespacedName, pipeline)).To(Succeed())
			}
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {

			crawlerResource := &v1beta1.Crawler{}
			err := k8sClient.Get(ctx, crawlerTypeNamespacedName, crawlerResource)
			Expect(err).NotTo(HaveOccurred())

			resource := &v1beta1.Search{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			childPipeline := &v1beta1.Pipeline{}
			err = k8sClient.Get(ctx, pipelineTypeNamespacedName, childPipeline)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Search")
			Expect(k8sClient.Delete(ctx, childPipeline)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, crawlerResource)).To(Succeed())

		})
		It("Should only mark complete if child resources are also complete", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SearchReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				SearchClusterRole: "test-search-cluster-role",
				SidecarImage:      "ocular-sidecar:test",
				SidecarPullPolicy: corev1.PullNever,
			}

			// first ensure reconcile is working normally
			// run reconcile twice and check pods are started
			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})).Error().NotTo(HaveOccurred())

			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})).Error().NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, search)).Error().ToNot(HaveOccurred())

			Expect(search.Status.StartTime).ToNot(BeNil())
			Expect(search.Status.CompletionTime).To(BeNil())

			searchPods := &corev1.PodList{}
			err := k8sClient.List(ctx, searchPods, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					v1beta1.TypeLabelKey:   v1beta1.PodTypeSearch,
					v1beta1.SearchLabelKey: search.Name,
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(searchPods.Items).To(HaveLen(1))
			searchPod := searchPods.Items[0]
			searchPod.Status.Phase = corev1.PodSucceeded
			Expect(k8sClient.Status().Update(ctx, &searchPod)).Error().ToNot(HaveOccurred())

			// run once more to check handleCompletion doesn't mark
			// as complete till pipeline is marked
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			// Expect(result.RequeueAfter).ToNot(BeZero())
			Expect(k8sClient.Get(ctx, typeNamespacedName, search)).Error().ToNot(HaveOccurred())
			Expect(search.Status.StartTime).ToNot(BeNil())
			Expect(search.Status.CompletionTime).To(BeNil())

			By("Updating the child resource to complete")
			pipeline.Status.CompletionTime = ptr.To(metav1.NewTime(time.Now()))
			Expect(k8sClient.Status().Update(ctx, pipeline)).Error().ToNot(HaveOccurred())

			Expect(controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})).Error().NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, search)).Error().ToNot(HaveOccurred())
			Expect(search.Status.StartTime).ToNot(BeNil())
			Expect(search.Status.CompletionTime).ToNot(BeNil())

		})
	})
})
