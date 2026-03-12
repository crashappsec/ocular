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

	"github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Search Webhook", func() {
	rnd := rand.New(rand.NewSource(GinkgoRandomSeed()))
	var (
		suffix    = testutils.GenerateRandomString(rnd, 5, testutils.LowercaseAlphabeticLetterSet)
		obj       *v1beta1.Search
		oldObj    *v1beta1.Search
		crawler   *v1beta1.Crawler
		customSA  *v1.ServiceAccount
		validator SearchCustomValidator
		defaulter SearchCustomDefaulter
	)

	BeforeEach(func() {
		crawler = &v1beta1.Crawler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "crawler",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1beta1.CrawlerSpec{
				Parameters: []v1beta1.ParameterDefinition{
					{
						Name:     "PARAM_1",
						Required: true,
					},
					{
						Name:     "PARAM_2",
						Required: false,
						Default:  ptr.To("parameter 2"),
					},
					{
						Name:     "PARAM_3",
						Required: false,
						Default:  ptr.To("parameter 3"),
					},
				},
			},
		}
		obj = &v1beta1.Search{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "search-" + suffix,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1beta1.SearchSpec{
				CrawlerRef: v1beta1.ParameterizedObjectReference{
					ObjectReference: v1.ObjectReference{
						Name:      crawler.Name,
						Namespace: crawler.Namespace,
					},
					Parameters: []v1beta1.ParameterSetting{
						{
							Name:  "PARAM_1",
							Value: "value 1",
						},
						{
							Name:  "PARAM_3",
							Value: "override value 3",
						},
					}},
			},
		}
		customSA = &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-test-custom-sa",
				Namespace: metav1.NamespaceDefault,
			},
		}
		oldObj = &v1beta1.Search{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "search-" + suffix,
				Namespace: metav1.NamespaceDefault,
			},
		}
		validator = SearchCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(crawler).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	JustAfterEach(func() {
		var err error
		if crawler != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, crawler))
			Expect(err).ToNot(HaveOccurred())
		}
		if customSA != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, customSA))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When creating a search", func() {
		It("Should default the service account when non is given", func() {
			Expect(defaulter.Default(ctx, obj)).Error().To(Succeed())
			Expect(obj.Spec.ServiceAccountName).To(Equal("search-" + obj.Name))
			Expect(obj.Status.CustomServiceAccount).To(BeFalse())
		})
		It("Should indicate custom service account when name is set by user", func() {
			objSA := obj.DeepCopy()
			objSA.Spec.ServiceAccountName = customSA.Name
			Expect(defaulter.Default(ctx, objSA)).Error().To(Succeed())
			Expect(objSA.Spec.ServiceAccountName).To(Equal(customSA.Name))
			Expect(objSA.Status.CustomServiceAccount).To(BeTrue())
		})

		It("Should return an error, if the crawler doesn't exist", func() {
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})

		It("Should succeed if the crawler exists", func() {
			Expect(k8sClient.Create(ctx, crawler)).To(Succeed())
			Expect(validator.ValidateCreate(ctx, obj)).Error().To(Succeed())
		})

		It("Should fail if the custom service account doesn't exist", func() {
			Expect(k8sClient.Create(ctx, crawler)).To(Succeed())
			objSA := obj.DeepCopy()
			objSA.Spec.ServiceAccountName = customSA.Name
			objSA.Status.CustomServiceAccount = true
			_, err := validator.ValidateCreate(ctx, objSA)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})

		It("Should succeed if the custom service account exists", func() {
			Expect(k8sClient.Create(ctx, crawler)).To(Succeed())
			Expect(k8sClient.Create(ctx, customSA)).To(Succeed())
			objSA := obj.DeepCopy()
			objSA.Spec.ServiceAccountName = customSA.Name
			objSA.Status.CustomServiceAccount = true
			Expect(validator.ValidateCreate(ctx, objSA)).Error().To(Succeed())
		})

		It("Should return an error if the CrawlerRef.Namespace is set to a different namespace", func() {
			objNS := obj.DeepCopy()
			objNS.Spec.CrawlerRef.Namespace = "different-namespace"
			_, err := validator.ValidateCreate(ctx, objNS)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})

		It("Should validate the parameters if the crawler exists and parameters are defined", func() {
			Expect(k8sClient.Create(ctx, crawler)).To(Succeed())
			obj.Spec.CrawlerRef.Parameters = []v1beta1.ParameterSetting{
				{
					Name:  "PARAM_2",
					Value: "value 2",
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})
	})

	Context("When updating Search under Validating Webhook", func() {
		It("Should return an error, if the new crawler doesn't exist", func() {
			obj.Spec.CrawlerRef.Name = "non-existent-crawler"
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})

		It("Should validate the parameters if the crawler exists and parameters are defined", func() {
			obj.Spec.CrawlerRef.Parameters = []v1beta1.ParameterSetting{
				{
					Name:  "PARAM_3",
					Value: "value 3",
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		})
	})

})
