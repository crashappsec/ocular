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
	"encoding/base64"
	"encoding/json"
	"fmt"

	errs "github.com/crashappsec/ocular/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// SecretStore is a Backend that uses a Kubernetes secret
// to store and retrieve objects.
type SecretStore[T Object] struct {
	name string
	s    typedCorev1.SecretInterface
}

var _ Backend[dummyObject] = &SecretStore[dummyObject]{}

// NewSecretStore creates a new SecretStore with the given name and defaults.
func NewSecretStore[T Object](
	name string,
	s typedCorev1.SecretInterface,
) *SecretStore[T] {
	return &SecretStore[T]{
		name: name,
		s:    s,
	}
}

func (c *SecretStore[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	if key == "" {
		return t, errs.New(errs.TypeBadRequest, nil, "a key must be specified")
	}

	// TODO(bryce): cache via mounting the secret
	storage, err := c.s.Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return t, fmt.Errorf("error getting value for key '%s': %w", key, err)
	}

	data, exists := storage.Data[key]
	if !exists {
		return t, errs.New(errs.TypeNotFound, nil, "value for key '%s' not found", key)
	}

	if err = unmarshallObject(data, &t); err != nil {
		zap.L().
			Error("error unmarshalling secret store data for key", zap.String("key", key), zap.Error(err))
		return t, fmt.Errorf("invalid secret value for key '%s'", key)
	}

	return t, nil
}

func (c *SecretStore[T]) Set(ctx context.Context, key string, value T) error {
	marshalled, err := marshallObject(value)
	if err != nil {
		return errs.New(errs.TypeBadRequest, nil, "failed to marshal object to secret value")
	}

	encodedValue := base64.StdEncoding.EncodeToString(marshalled)

	mergePatchOps := map[string]interface{}{
		"data": map[string]interface{}{
			key: encodedValue,
		},
	}

	// convert the patch to JSON
	patchBytes, err := json.Marshal(mergePatchOps)
	if err != nil {
		return err
	}

	patchType := k8sTypes.MergePatchType
	_, err = c.s.Patch(
		ctx,
		c.name,
		patchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	return err
}

func (c *SecretStore[T]) Delete(ctx context.Context, key string) error {
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
	_, err = c.s.Patch(
		ctx,
		c.name,
		patchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	return err
}

func (c *SecretStore[T]) List(ctx context.Context) (map[string]T, error) {
	profileStore, err := c.s.Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	m := make(map[string]T, len(profileStore.Data))

	for key, raw := range profileStore.Data {
		var t T
		if err = unmarshallObject(raw, &t); err != nil {
			zap.L().
				Error("error unmarshalling secret store data for key ", zap.String("key", key), zap.Error(err))
			return nil, fmt.Errorf("invalid secret value for key '%s'", key)
		}
		m[key] = t
	}

	return m, nil
}
