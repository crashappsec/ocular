// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crashappsec/ocular/api/v1beta1"
)

var _ = Describe("Uploader Webhook", func() {
	var (
		obj       *v1beta1.Uploader
		profile   *v1beta1.Profile
		validator UploaderCustomValidator
		namespace = "default"
	)

	BeforeEach(func() {
		obj = &v1beta1.Uploader{
			TypeMeta: metav1.TypeMeta{
				Kind: "Uploader",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uploader-webhook-test",
				Namespace: namespace,
			},
			Spec: v1beta1.UploaderSpec{
				Container: v1.Container{
					Name:  "dummy-uploader-container",
					Image: "dummy-uploader-image",
				},
			},
		}
		profile = &v1beta1.Profile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uploader-webhook-test-profile",
				Namespace: namespace,
			},
			Spec: v1beta1.ProfileSpec{
				UploaderRefs: []v1beta1.ParameterizedLocalObjectReference{
					{
						Name: obj.Name,
						Kind: "Uploader",
					},
				},
				Containers: []v1beta1.ConditionalContainer{
					{
						Container: v1.Container{
							Name:  "dummy-scan-container",
							Image: "dummy-scan-image",
						},
					},
				},
			},
		}
		validator = UploaderCustomValidator{
			c: k8sClient,
		}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		Expect(profile).NotTo(BeNil(), "Expected profileRef to be initialized")
	})

	AfterEach(func() {
		var err error
		if profile != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, profile))
			Expect(err).ToNot(HaveOccurred())
		}
		if obj != nil {
			err = ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, obj))
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("When creating or updating Uploader under Validating Webhook", func() {
		// Create tests
		// NOTE: create is currently not registered

		// Update tests
		It("Should validate newly required params validated for references", func() {
			By("creating a Profile that references the Uploader, then updating Uploader to add a new required param")
			obj.Spec.Parameters = []v1beta1.ParameterDefinition{
				{Name: "param1"},
				{Name: "param2", Default: ptr.To("default_value")},
			}
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			var uploader v1beta1.Uploader
			Expect(k8sClient.Get(ctx, ctrlclient.ObjectKeyFromObject(obj), &uploader)).To(Succeed())

			profile.Spec.UploaderRefs = []v1beta1.ParameterizedLocalObjectReference{
				{
					Name: obj.Name,
					Kind: "Uploader",
					Parameters: []v1beta1.ParameterSetting{
						{Name: "param1", Value: "value1"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			oldObj := uploader.DeepCopy()
			uploader.Spec.Parameters = []v1beta1.ParameterDefinition{
				{Name: "param1"},
				{Name: "param2", Default: ptr.To("default_value")},
				{Name: "param3"},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, &uploader)
			Expect(err).To(HaveOccurred(), "Expected validation to fail due to new required parameter not being set in Profile reference")

			By("updating the Profile to include the new required param")
			profile.Spec.UploaderRefs[0].Parameters = append(profile.Spec.UploaderRefs[0].Parameters, v1beta1.ParameterSetting{
				Name:  "param3",
				Value: "value3",
			})
			Expect(k8sClient.Update(ctx, profile)).To(Succeed())

			Expect(validator.ValidateUpdate(ctx, oldObj, &uploader)).To(BeNil(), "Expected validation to pass after Profile reference updated with new required parameter")
		})

		// Delete tests
		It("Should not allow deletion of Uploader if referenced by Profile", func() {
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())

			By("creating a Profile that references the Uploader, then attempting to delete the Uploader")
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			var uploader v1beta1.Uploader
			Expect(k8sClient.Get(ctx, ctrlclient.ObjectKeyFromObject(obj), &uploader)).To(Succeed())

			_, err := validator.ValidateDelete(ctx, &uploader)
			Expect(apierrors.IsForbidden(err)).To(BeTrue(), "Expected validation to fail due to existing Profile referencing Uploader")

			By("deleting the Profile, then attempting to delete the Uploader again")
			Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
			Expect(validator.ValidateDelete(ctx, obj)).To(BeNil(), "Expected validation to pass after Profile referencing Uploader is deleted")
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed(), "Expected Uploader deletion to succeed after Profile is deleted")
		})
	})

})
