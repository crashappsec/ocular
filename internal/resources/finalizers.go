// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	"context"
	"fmt"
	"slices"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type FinalizerObject interface {
	// functions from v1.TypeMeta //

	// GetObjectKind returns the ObjectKind of the object.
	// from [k8s.io/apimachinery/pkg/apis/meta/v1.TypeMeta]
	GetObjectKind() schema.ObjectKind

	// functions from v1.ObjectMeta //

	// GetName returns the name of the object.
	// From [k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta]
	GetName() string

	// GetDeletionTimestamp returns the deletion timestamp
	// if the object has been deleted. If the object hasn't been
	// deleted, nil is returned.
	// From [k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta]
	GetDeletionTimestamp() *v1.Time

	// GetFinalizers returns the finalizers of the object.
	// From [k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta]
	GetFinalizers() []string

	// SetFinalizers sets the finalizers of the object.
	// From [k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta]
	SetFinalizers(finalizers []string)
}

func PerformFinalizer[T FinalizerObject](ctx context.Context, t T, finalizerId string, finalizer func(ctx context.Context, t T) error) (bool, error) {
	l := log.FromContext(ctx)

	kind := t.GetObjectKind().GroupVersionKind().Kind

	finalizers := t.GetFinalizers()
	isDownloaderMarkedForDeletion := t.GetDeletionTimestamp() != nil
	if isDownloaderMarkedForDeletion {
		if slices.Contains(finalizers, finalizerId) {
			l.Info(fmt.Sprintf("performing finalizer cleanup for %s %s", kind, t.GetName()), "name", t.GetName(), "kind", kind)
			if finalizer != nil {
				if err := finalizer(ctx, t); err != nil {
					return false, err
				}
			}
			t.SetFinalizers(slices.DeleteFunc(finalizers, func(s string) bool {
				return strings.EqualFold(finalizerId, s)
			}))
			return true, nil
		}
		return false, nil
	}

	return false, nil
}
