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
	"fmt"
	"maps"
	"strings"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/resources"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// updateStatus will update the status of any [sigs.k8s.io/controller-runtime/pkg/client.Object]
// using the provided controller-runtime client. It will log any errors encountered during the update process
func updateStatus(ctx context.Context, c client.Client, obj client.Object, errorMsgKeysAndValues ...any) error {
	l := logf.FromContext(ctx)
	if updateErr := c.Status().Update(ctx, obj); updateErr != nil {
		l.Error(updateErr, fmt.Sprintf("failed to update status of %s", obj.GetName()), errorMsgKeysAndValues...)
		return updateErr
	}
	return nil
}

func generateBaseContainerOptions(envVars []corev1.EnvVar) []resources.ContainerOption {
	return []resources.ContainerOption{
		resources.ContainerWithAdditionalEnvVars(envVars...),
		resources.ContainerWithPodSecurityStandardRestricted(),
	}
}

func reconcilePodFromLabel[T client.Object](
	ctx context.Context,
	k8sclient client.Client,
	scheme *runtime.Scheme,
	owner T,
	pod *corev1.Pod,
	selectorLabels []string,
	metric prometheus.Counter,
) (*corev1.Pod, error) {
	if pod == nil {
		return nil, nil
	}

	l := logf.FromContext(ctx)
	l.Info("reconciling pod", "name", pod.GetName(), "labels", selectorLabels)

	var podList corev1.PodList

	labelSet := make(labels.Set)
	for _, label := range selectorLabels {
		labelSet[label] = pod.Labels[label]
	}

	listOpts := &client.ListOptions{
		Namespace:     owner.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(labelSet),
	}

	if err := k8sclient.List(ctx, &podList, listOpts); err != nil {
		l.Error(err, "error listing pods", "labels", selectorLabels)
		return nil, err
	}
	if len(podList.Items) == 0 {
		l.Info("no pods found matching labels, creating pod", "labels", selectorLabels)
		if err := ctrl.SetControllerReference(owner, pod, scheme); err != nil {
			l.Error(err, "error setting controller reference on pod", "name", pod.GetName())
			return nil, err
		}
		if err := k8sclient.Create(ctx, pod, &client.CreateOptions{}); err != nil {
			l.Error(err, "error creating pod", "name", pod.GetName())
			return nil, err
		}
		metric.Add(1)
		return pod, nil
	} else if len(podList.Items) > 1 {
		l.Info("multiple pods found matching labels, using the first one and deleting others", "count", len(podList.Items), "labels", selectorLabels)
		// delete all but the first one
		for _, p := range podList.Items[1:] {
			if err := k8sclient.Delete(ctx, &p, &client.DeleteOptions{}); err != nil {
				l.Error(err, "error deleting extra pod", "name", p.GetName())
				return nil, err
			}
		}
		return &podList.Items[0], nil
	}
	return &podList.Items[0], nil
}

func generateChildLabels(parents ...client.Object) map[string]string {
	childLabels := make(map[string]string)
	for _, parent := range parents {
		maps.Copy(childLabels, parent.GetLabels())
	}
	// we want to remove any existing ocular controller labels to avoid conflicts
	// or incorrect labeling
	maps.DeleteFunc(childLabels, func(k string, _ string) bool {
		return strings.HasPrefix(k, v1beta1.Group)
	})

	childLabels["app.kubernetes.io/managed-by"] = "ocular-controller"
	return childLabels
}
