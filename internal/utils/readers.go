// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utils

import (
	"context"
	"errors"
	"fmt"
	"os"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Closer interface {
	Close() error
}

// CloseAndLog is a utility function that closes a resource and logs any error that occurs.
// It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseAndLog(ctx context.Context, c Closer, msg string, keysAndValues ...any) {
	l := logf.FromContext(ctx)
	if err := c.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		l.Error(fmt.Errorf("error closing reader: %w", err), msg, keysAndValues...)
	}
}

type CloserIgnore[I any] interface {
	Close() (I, error)
}

// CloseIgnoreAndLog is a utility function that closes a resource and logs any error that occurs, and ignores
// the result. It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseIgnoreAndLog[I any](ctx context.Context, c CloserIgnore[I]) {
	l := logf.FromContext(ctx)
	if _, err := c.Close(); err != nil {
		l.Error(err, "failed to close")
	}
}
