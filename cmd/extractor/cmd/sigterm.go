// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func AwaitSigterm(ctx context.Context) {
	l := log.FromContext(ctx)
	sigTerm := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)
	l.Info("awaiting SIGTERM")
	sig := <-sigTerm
	l.Info("Received signal", "signal", sig.String())
}
