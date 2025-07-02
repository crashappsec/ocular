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
)

//go:generate mockgen -destination ../../internal/unittest/mocks/pkg/storage/backend.go -package=storage -typed . Backend

// Backend is the interface for a storage backend.
// A storage backend is responsible for storing and retrieving
// configurations or data defined by the user.
type Backend[T Object] interface {
	// Get retrieves the value associated with the given key.
	Get(ctx context.Context, key string) (T, error)
	// Set stores the value associated with the given key.
	// The Object should have the method [Object.Validate] called before this.
	Set(ctx context.Context, key string, value T) error
	// Delete removes the value associated with the given key.
	Delete(ctx context.Context, key string) error
	// List returns a list of all keys in the storage.
	List(ctx context.Context) (map[string]T, error)
}
