// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Sidecar is a utility binary that wraps user process in containers
// and allows for the uploaders to await scanner completion before starting.
// First the sidecar runs as an init container and copies itself to a shared volume.
// then it will wrap the scanner containers to write their exit code to a file
// with the name as the scanner container. Next the wrapped uploaders wait until
// all scanners finish before starting.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/process"
	"golang.org/x/sync/errgroup"
)

var (
	version   = "unknown"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	ctx := context.Background()
	l := slog.With(
		slog.String("version", version),
		slog.String("git-commit", gitCommit),
		slog.String("build-time", buildTime),
	)

	l.Info("starting ocular sidecar")
	if len(os.Args) < 2 {
		l.Error("no command specified for sidecar")
		fmt.Println("Usage: sidecar <command> [user command...]")
		os.Exit(1)
	}

	var (
		command = os.Args[1]
		userCmd = os.Args[2:]
	)

	cancelCtx, awaitSigterm := process.CancelContextSigterm(ctx)
	go awaitSigterm()

	l = l.With(slog.Any("userCmd", userCmd), slog.String("command", command))
	l.Info("starting sidecar command " + command)
	switch command {
	case "init":
		execPath, err := os.Executable()
		if err != nil {
			l.Error("unable to determine executable path", slog.Any("error", err))
			os.Exit(1)
		}
		runtimePath := os.Getenv(v1beta1.EnvVarSidecarPath)
		err = process.CopyFile(ctx, execPath, runtimePath)
		if err != nil {
			l.Error("failed to copy executable", slog.Any("error", err))
			os.Exit(1)
		}
		err = os.Chmod(runtimePath, 0o755)
		if err != nil {
			l.Error("unable to change permissions of executable", slog.Any("error", err))
		}
		err = os.MkdirAll(os.Getenv(v1beta1.EnvVarProcessDir), 0o777)
		if err != nil {
			l.Error("unable to create process directory ", slog.Any("error", err))
		}
	case "await-scanners":
		cmd, err := process.BuildUserCommand(cancelCtx, userCmd)
		if err != nil {
			l.Error("unable to parse user command", slog.Any("error", err))
			os.Exit(1)
		}
		awaitScannersHook := AwaitScans(strings.Split(os.Getenv(v1beta1.EnvVarScanContainerNames), ","))
		exitCode, err := process.HookCommand(cancelCtx, cmd, awaitScannersHook, nil)
		if err != nil {
			l.Error("unable to execute scanner", slog.Any("error", err))
			os.Exit(1)
		}
		os.Exit(exitCode)
	case "scanner":
		cmd, err := process.BuildUserCommand(cancelCtx, userCmd)
		if err != nil {
			l.Error("unable to parse user command", slog.Any("error", err))
			os.Exit(1)
		}
		exitCode, err := process.HookCommand(cancelCtx, cmd, nil, ScanCompleteHook(os.Getenv(v1beta1.EnvVarContainerName)))
		if err != nil {
			l.Error("unable to execute scanner", slog.Any("error", err))
			os.Exit(1)
		}
		os.Exit(exitCode)
	default:
		slog.Error("unknown command")
		os.Exit(1)
	}
}

func AwaitScans(scanners []string) process.Hook {
	return func(ctx context.Context, _ *exec.Cmd) error {
		g, ctx := errgroup.WithContext(ctx)
		for _, scanner := range scanners {
			l := slog.With(slog.String("scanner", scanner))
			l.Info("awating scanner")
			g.Go(func() error {
				for {
					complete, err := IsScannerComplete(ctx, scanner)
					if err != nil {
						l.Error("unable to check for scanner completion", slog.Any("error", err))
						return err
					}

					if complete {
						l.Info("scanner complete")
						return nil
					}
					l.Info("scanner not complete, sleeping before checking again")
					time.Sleep(time.Second)
				}
			})
		}
		return g.Wait()
	}
}

func IsScannerComplete(_ context.Context, scanner string) (bool, error) {
	markPath := path.Join(os.Getenv(v1beta1.EnvVarProcessDir), scanner)
	_, err := os.Stat(markPath)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func ScanCompleteHook(scanner string) process.Hook {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		markPath := path.Join(os.Getenv(v1beta1.EnvVarProcessDir), scanner)

		exitcode := cmd.ProcessState.ExitCode()
		f, err := os.Create(markPath)
		if err != nil {
			return fmt.Errorf("unable to create mark path '%s' for scanner %s: %w", markPath, scanner, err)
		}

		_, err = f.WriteString(strconv.Itoa(exitcode))
		if err != nil {
			return fmt.Errorf("unable to write exit code '%d' for scanner %s: %w", exitcode, scanner, err)
		}

		return nil
	}
}
