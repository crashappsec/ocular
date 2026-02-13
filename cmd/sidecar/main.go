// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Utility image to transfer files between scanners and uploaders.
// It will run as both a sidecar container for the scanner job and
// an init container for the uploader job. When the scanner job is finished,
// it will receive a SIGTERM to stop the container and upload the files to the
// specified uploader job. On the uploader job, it will run as an init container
// listening for the requests sent from the sidecar container of the scanner job.
// It will write all files received to the shared results directory and exit 0,
// allowing uploaders to run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crashappsec/ocular/cmd/sidecar/cmd"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	version   = "unknown"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	ctx := context.Background()

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&zap.Options{})).
		WithValues("version", version, "buildTime", buildTime, "gitCommit", gitCommit)
	logf.SetLogger(logger)
	ctx = logf.IntoContext(ctx, logger)

	logger.Info("starting ocular sidecar")
	if len(os.Args) < 2 {
		logger.Error(fmt.Errorf("no command specified"), "no command specified for sidecar")
		fmt.Println("Usage: sidecar <command> [args...]")
		os.Exit(1)
	}

	var (
		command = os.Args[1]
		err     error
	)

	var files []string
	for n, arg := range os.Args {
		if arg == "--" {
			files = os.Args[n+1:]
			break
		}
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	logger.Info("starting sidecar in mode "+command, "files", files, "command", command)
	switch command {
	case "receive":
		err = cmd.Receive(cancelCtx, files)
	case "extract":
		awaitSigterm(cancelCtx, cancel)
		err = cmd.Extract(cancelCtx, files)
	case "scheduler":
		go awaitSigterm(cancelCtx, cancel)
		if err := cmd.Schedule(cancelCtx); err != nil {
			logger.Error(err, "unable to run scheduler")
		}
	case "scheduler-keepalive": // keeps pod alive till scheduler is done
		awaitFIFOs(ctx, false)
		os.Exit(0)
	case "scheduler-init": // awaits scheduler to be ready before existing
		awaitFIFOs(ctx, true)
		os.Exit(0)
	case "ignore":
		logger.Info("no uploaders specified, ignoring files and shutting down gracefully")
		awaitSigterm(cancelCtx, cancel)
	default:
		err = fmt.Errorf("unknown argument: %s", command)
	}

	if err != nil {
		fmt.Println("error:", err)
		logger.Error(err, "failed to extract files", "command", command)
		os.Exit(1)
	}
}

func awaitSigterm(ctx context.Context, cancel context.CancelFunc) {
	l := logf.FromContext(ctx)
	sigTerm := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)
	l.Info("awaiting SIGTERM")
	sig := <-sigTerm
	cancel()
	l.Info("Received signal", "signal", sig.String())
}

// awaitFIFOs will await both the search and pipeline
// FIFO files exist if exists is true, otherwise it
// will await they dont exist.
func awaitFIFOs(ctx context.Context, exists bool) {
	var searchFIFO, pipelineFIFO bool
	for {
		_, err := cmd.StatPipelineFIFO(ctx)
		if exists {
			pipelineFIFO = err != nil
		} else {
			pipelineFIFO = errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrClosed)
		}
		_, err = cmd.StatSearchFIFO(ctx)
		if exists {
			searchFIFO = err != nil
		} else {
			searchFIFO = errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrClosed)
		}

		if pipelineFIFO && searchFIFO {
			return
		}
		time.Sleep(time.Second)
	}
}
