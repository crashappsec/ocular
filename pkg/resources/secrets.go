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

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/crashappsec/ocular/pkg/storage"
	"gopkg.in/yaml.v3"
)

type Secret schemas.Secret

func (s Secret) Validate(ctx context.Context, _ cluster.Context) error {
	return nil
}

func (s *Secret) UnmarshalYAML(node *yaml.Node) error {
	if s == nil {
		return fmt.Errorf("yaml.RawMessage: UnmarshalYAML on nil pointer")
	}
	*s = append((*s)[0:0], []byte(node.Value)...)
	return nil
}

func (s Secret) MarshalText() (text []byte, err error) {
	return s, nil
}

func (s *Secret) UnmarshalText(_ []byte) error {
	if s == nil {
		return fmt.Errorf("UnmarshalText on nil pointer")
	}
	*s = []byte("**[VALUE MASKED]**")
	return nil
}

type SecretStorageBackend = storage.Backend[Secret]

func NewSecretStorageBackend(clusterCtx cluster.Context) SecretStorageBackend {
	cm := storage.NewSecretStore[Secret](
		config.State.Secrets.SecretName,
		clusterCtx.CS.CoreV1().Secrets(clusterCtx.Namespace))
	return cm
}
