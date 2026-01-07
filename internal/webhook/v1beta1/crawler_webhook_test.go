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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	ocularcrashoverriderunv1beta1 "github.com/crashappsec/ocular/api/v1beta1"

	testutils "github.com/crashappsec/ocular/test/utils"
)

var _ = Describe("Crawler Webhook", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		namespace = "default"
		search    *ocularcrashoverriderunv1beta1.Search
		obj       *ocularcrashoverriderunv1beta1.Crawler
		oldObj    *ocularcrashoverriderunv1beta1.Crawler
		validator CrawlerCustomValidator
	)

	BeforeEach(func() {
		obj = &ocularcrashoverriderunv1beta1.Crawler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crawler",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.CrawlerSpec{
				Container: testutils.GenerateRandomContainer(rnd),
			},
		}
		oldObj = &ocularcrashoverriderunv1beta1.Crawler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crawler",
				Namespace: namespace,
			},
		}
		search = &ocularcrashoverriderunv1beta1.Search{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crawler-webhook-test-search",
				Namespace: namespace,
			},
			Spec: ocularcrashoverriderunv1beta1.SearchSpec{
				CrawlerRef: ocularcrashoverriderunv1beta1.CrawlerObjectReference{
					ObjectReference: v1.ObjectReference{
						Name: obj.Name,
					},
				},
			},
		}
		validator = CrawlerCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		Expect(search).NotTo(BeNil(), "Expected search to be initialized")
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, search)
		_ = k8sClient.Delete(ctx, obj)
	})

	Context("When updating Crawler under Validating Webhook", func() {
		It("Should validate newly required params validated for references", func() {
			By("creating a Search that references the Crawler, then updating Crawler to add a new required param")
			oldObj.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{
				{Name: "param1", Required: true},
				{Name: "param2", Required: false, Default: ptr.To("default_value")},
			}
			Expect(k8sClient.Create(ctx, oldObj)).To(Succeed())

			search.Spec.CrawlerRef = ocularcrashoverriderunv1beta1.CrawlerObjectReference{
				ObjectReference: v1.ObjectReference{
					Name: oldObj.Name,
				},
				Parameters: []ocularcrashoverriderunv1beta1.ParameterSetting{
					{Name: "param1", Value: "value1"},
				},
			}
			Expect(k8sClient.Create(ctx, search)).To(Succeed())

			obj.Spec.Parameters = []ocularcrashoverriderunv1beta1.ParameterDefinition{
				{Name: "param1", Required: true},
				{Name: "param2", Required: false, Default: ptr.To("default_value")},
				{Name: "param3", Required: true},
			}
			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).Error().To(HaveOccurred(), "Expected validation to fail due to new required parameter not being set in Search reference")

			By("updating the Search to include the new required param")
			search.Spec.CrawlerRef.Parameters = append(search.Spec.CrawlerRef.Parameters, ocularcrashoverriderunv1beta1.ParameterSetting{
				Name:  "param3",
				Value: "value3",
			})
			Expect(k8sClient.Update(ctx, search)).To(Succeed())

			Expect(validator.ValidateUpdate(ctx, oldObj, obj)).To(BeNil(), "Expected validation to pass after Search reference updated with new required parameter")
		})
	})

	Context("When deleting a Crawler under Validating Webhook", func() {
		It("Should deny deletion if referenced by a Search", func() {
			By("Creating a Search that references the Crawler")
			Expect(k8sClient.Create(ctx, obj)).Should(Succeed())
			Expect(k8sClient.Create(ctx, search)).Should(Succeed())
			By("simulating a deletion scenario")
			_, err := validator.ValidateDelete(ctx, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})
	})

})
