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

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// patchResource will patch any [sigs.k8s.io/controller-runtime/pkg/client.Object]
// using the provided controller-runtime client.
// The caller should create a patch for the resource using
// [sigs.k8s.io/controller-runtime/pkg/client.MergeFrom] (making sure to create a deep copy),
// update the resource, then call this function.
// It will log any errors encountered during the update process
func patchResource(ctx context.Context, c client.Client, obj client.Object, patch client.Patch) error {
	l := logf.FromContext(ctx)
	if patchErr := c.Patch(ctx, obj, patch); patchErr != nil {
		gvk := "unknown"
		if objKind := obj.GetObjectKind(); objKind != nil {
			gvk = objKind.GroupVersionKind().String()
		}
		l.Error(patchErr, "failed to patch resource",
			"name", obj.GetName(), "namespace", obj.GetNamespace(),
			"gvk", gvk)
		return patchErr
	}
	return nil
}

// patchStatus will patch any [sigs.k8s.io/controller-runtime/pkg/client.Object]
// status subresource using the provided controller-runtime client.
// The caller should create a patch for the resource using
// [sigs.k8s.io/controller-runtime/pkg/client.MergeFrom] (making sure to create a deep copy),
// update the resource's status, then call this function.
// It will log any errors encountered during the update process
func patchStatus(ctx context.Context, c client.Client, obj client.Object, patch client.Patch) error {
	l := logf.FromContext(ctx)
	if patchErr := c.Status().Patch(ctx, obj, patch); patchErr != nil {
		gvk := "unknown"
		if objKind := obj.GetObjectKind(); objKind != nil {
			gvk = objKind.GroupVersionKind().String()
		}
		l.Error(patchErr, "failed to patch status resource",
			"name", obj.GetName(), "namespace", obj.GetNamespace(),
			"gvk", gvk)
		return patchErr
	}
	return nil
}
