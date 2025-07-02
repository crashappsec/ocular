// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import (
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// ServiceOpt is a function that modifies a Service object.
type ServiceOpt = func(v1.Service) v1.Service

// ServiceOptPublishNotReadyAddresses sets the 'PublishNotReadyAddresses' field of the Service object.
func ServiceOptPublishNotReadyAddresses(publishNotReadyAddresses bool) ServiceOpt {
	return func(svc v1.Service) v1.Service {
		svc.Spec.PublishNotReadyAddresses = publishNotReadyAddresses
		return svc
	}
}

// BuildServiceForJob creates a Service object for a Job.
// It sets the name, namespace, and owner reference of the Service
// to match the Job. It also sets the selector to match the Job's labels.
func BuildServiceForJob(
	job *batchV1.Job,
	namespace string,
	port int32,
	opts ...ServiceOpt,
) v1.Service {
	svc := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.Name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: batchV1.SchemeGroupVersion.String(),
					Kind:       "Job",
					Name:       job.Name,
					UID:        job.UID,
					Controller: ptr.To[bool](true),
				},
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"job-name": job.Name,
			},
			Ports: []v1.ServicePort{
				{Port: port, TargetPort: intstr.FromInt32(port)},
			},
		},
	}
	for _, opt := range opts {
		svc = opt(svc)
	}

	return svc
}
