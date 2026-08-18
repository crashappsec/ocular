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
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func CancelContextSigterm(ctx context.Context) (context.Context, func()) {
	cancelCtx, cancel := context.WithCancel(ctx)
	sigTerm := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)
	return cancelCtx, func() {
		slog.Info("awaiting SIGTERM")
		sig := <-sigTerm
		slog.Info("received cancel signal, cancelling context", "signal", sig.String())
		cancel()
	}
}
