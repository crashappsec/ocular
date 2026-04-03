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
	"github.com/crashappsec/ocular/internal/containers"
	corev1 "k8s.io/api/core/v1"
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

func patchStatus(ctx context.Context, c client.Client, obj client.Object, patch client.Patch) error {
	l := logf.FromContext(ctx)
	if patchErr := c.Status().Patch(ctx, obj, patch); patchErr != nil {
		l.Error(patchErr, fmt.Sprintf("failed to patch status of %s", obj.GetName()))
		return patchErr
	}
	return nil
}

func generateBaseContainerOptions(envVars []corev1.EnvVar) []containers.Option {
	return []containers.Option{
		containers.WithAdditionalEnvVars(envVars...),
		containers.WithPodSecurityStandardRestricted(),
	}
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
