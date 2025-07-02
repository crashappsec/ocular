// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func AwaitSigterm() {
	sigTerm := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)
	zap.L().Info("awaiting SIGTERM")
	sig := <-sigTerm
	zap.L().Info("Received signal", zap.String("signal", sig.String()))
}
