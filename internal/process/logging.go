// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package process

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
)

// CloseAndLog is a utility function that closes a resource and logs any error that occurs.
// It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseAndLog(ctx context.Context, c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		slog.Error("error closing reader", slog.Any("error", err))
	}
}

type CloserIgnore[I any] interface {
	Close() (I, error)
}

// CloseIgnoreAndLog is a utility function that closes a resource and logs any error that occurs, and ignores
// the result. It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseIgnoreAndLog[I any](ctx context.Context, c CloserIgnore[I]) {
	if c == nil {
		return
	}
	if _, err := c.Close(); err != nil {
		slog.Error("failed to close and ignore", slog.Any("error", err))
	}
}

func RemovePathAndLog(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil {
		slog.Error("unable to remove path", slog.String("path", path), slog.Any("error", err))
	}
}
