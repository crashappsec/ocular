// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utilities

import "go.uber.org/zap"

type Closer interface {
	Close() error
}

// CloseAndLog is a utility function that closes a resource and logs any error that occurs.
// It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseAndLog(c Closer) {
	if err := c.Close(); err != nil {
		zap.L().Error("failed to close", zap.Error(err))
	}
}

// CallAndLogWithArg is a utility function that calls a function with an argument and logs any error that occurs.
// It is useful for defers to ensure that the function is called properly and any errors are logged.
func CallAndLogWithArg[A any](c func(A) error, arg A) {
	if err := c(arg); err != nil {
		zap.L().Error("failed to close", zap.Error(err))
	}
}

type CloserIgnore[I any] interface {
	Close() (I, error)
}

// CloseIgnoreAndLog is a utility function that closes a resource and logs any error that occurs, and ignores
// the result. It should be used for defers to ensure that the resource is closed properly and any errors are logged.
func CloseIgnoreAndLog[I any](c CloserIgnore[I]) {
	if _, err := c.Close(); err != nil {
		zap.L().Error("failed to close", zap.Error(err))
	}
}
