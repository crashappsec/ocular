// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package storage

import (
	"context"
	"encoding/json"
	"fmt"

	errs "github.com/crashappsec/ocular/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ConfigMap is a Backend that uses a Kubernetes config map
// to store and retrieve objects.
type ConfigMap[T Object] struct {
	name string
	cm   typedCorev1.ConfigMapInterface
}

var _ Backend[dummyObject] = &ConfigMap[dummyObject]{}

func NewConfigMap[T Object](
	name string,
	cm typedCorev1.ConfigMapInterface,
) *ConfigMap[T] {
	return &ConfigMap[T]{
		name: name,
		cm:   cm,
	}
}

func (c *ConfigMap[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	if key == "" {
		return t, errs.New(errs.TypeBadRequest, nil, "a profile must be specified")
	}

	// TODO(bryce): cache via mounting the configmap
	storage, err := c.cm.Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return t, fmt.Errorf("error getting profile: %w", err)
	}

	data, exists := storage.Data[key]
	if !exists {
		return t, errs.New(errs.TypeNotFound, nil, "value for key '%s' not found", key)
	}

	if err = unmarshallObject([]byte(data), &t); err != nil {
		return t, fmt.Errorf("invalid profile '%s' stored in config", key)
	}

	return t, nil
}

func (c *ConfigMap[T]) Set(ctx context.Context, key string, value T) error {
	marshalled, err := marshallObject(value)
	if err != nil {
		return errs.New(errs.TypeBadRequest, nil, "failed to marshal profile")
	}

	mergePatchOps := map[string]interface{}{
		"data": map[string]interface{}{
			key: string(marshalled),
		},
	}

	// convert the patch to JSON
	patchBytes, err := json.Marshal(mergePatchOps)
	if err != nil {
		return err
	}

	patchType := k8sTypes.MergePatchType
	_, err = c.cm.Patch(
		ctx,
		c.name,
		patchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	return err
}

func (c *ConfigMap[T]) Delete(ctx context.Context, key string) error {
	mergePathOps := map[string]any{
		"data": map[string]any{
			key: nil,
		},
	}

	patchBytes, err := json.Marshal(mergePathOps)
	if err != nil {
		return err
	}

	patchType := k8sTypes.MergePatchType
	_, err = c.cm.Patch(
		ctx,
		c.name,
		patchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	return err
}

func (c *ConfigMap[T]) List(ctx context.Context) (map[string]T, error) {
	profileStore, err := c.cm.Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	m := make(map[string]T, len(profileStore.Data))
	for key, raw := range profileStore.Data {
		var t T
		if err = unmarshallObject([]byte(raw), &t); err != nil {
			return nil, err
		}
		m[key] = t
	}

	return m, nil
}
