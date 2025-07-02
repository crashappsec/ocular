// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package unittest

import (
	"bytes"
	"io"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CaptureLogs sets up the logger to output to a buffer for testing purposes.
// This is useful for capturing logs during tests and verifying their content.
func CaptureLogs(t *testing.T) io.Reader {
	t.Helper()
	buffer := &bytes.Buffer{}
	globalRevert := zap.ReplaceGlobals(zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(buffer),
			zapcore.DebugLevel,
		)))
	t.Cleanup(globalRevert)
	return buffer
}

// EnableLogs sets up the logger to output to stderr for testing purposes.
// This is useful for debugging failures in tests.
func EnableLogs(t *testing.T) {
	t.Helper()
	globalRevert := zap.ReplaceGlobals(zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(os.Stderr),
			zapcore.DebugLevel,
		)))
	t.Cleanup(globalRevert)
}
