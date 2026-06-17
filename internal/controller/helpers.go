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

	"github.com/crashappsec/ocular/internal/containers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// updateStatus will update the status of any [sigs.k8s.io/controller-runtime/pkg/client.Object]
// using the provided controller-runtime client. It will log any errors encountered during the update process
func updateStatus(ctx context.Context, c client.Client, obj client.Object, errorMsgKeysAndValues ...any) error {
	l := logf.FromContext(ctx)
	if updateErr := c.Status().Update(ctx, obj); updateErr != nil {
		kind := schema.GroupVersionKind{Kind: "unknown"}
		if objKind := obj.GetObjectKind(); objKind != nil {
			kind = objKind.GroupVersionKind()
		}
		l.Error(updateErr, fmt.Sprintf("failed to update status for %s of type %s", obj.GetName(), kind),
			errorMsgKeysAndValues...)
		return updateErr
	}
	return nil
}

func patchResource(ctx context.Context, c client.Client, obj client.Object, patch client.Patch) error {
	l := logf.FromContext(ctx)
	if patchErr := c.Patch(ctx, obj, patch); patchErr != nil {
		kind := schema.GroupVersionKind{Kind: "unknown"}
		if objKind := obj.GetObjectKind(); objKind != nil {
			kind = objKind.GroupVersionKind()
		}
		l.Error(patchErr, fmt.Sprintf("failed to patch resource %s of type %s", obj.GetName(), kind))
		return patchErr
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
