// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package mocks will contain all the generated mocks for the unit tests.
// It will contain mocks for interfaces that are used in the codebase but
// defined in external packages. This is to avoid having to import
// external packages in the codebase. See the package [github.com/crashappsec/ocular/internal/unitest/pkg] for
// mocks of interfaces defined in the codebase.
package mocks

//go:generate mockgen -destination mock_typed_corev1.go -package=mocks -typed k8s.io/client-go/kubernetes/typed/core/v1 SecretInterface,ConfigMapInterface,ServiceInterface

//go:generate mockgen -destination mock_typed_batchv1.go -package=mocks -typed k8s.io/client-go/kubernetes/typed/batch/v1 JobInterface
