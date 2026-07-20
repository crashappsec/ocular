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
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// BuildUserCommand will return a [exec.Cmd] for the user command
// of the process. This is used when the process for a container is wrapped
// and meant to perform tasks before or after the user command runs.
func BuildUserCommand(ctx context.Context, userCmd []string) (*exec.Cmd, error) {
	if len(userCmd) == 0 {
		return nil, fmt.Errorf("no user command provided")
	}
	entrypoint, args := userCmd[0], userCmd[1:]
	cmd := exec.CommandContext(ctx, entrypoint, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd, nil
}

// Hook represents the
type Hook = func(context.Context, *exec.Cmd) error

// HookCommand provides a pre-start and post-stop hooks for a [exec.Cmd].
// It will execute the pre-start hook, then execute the command as a subprocess,
// then run the post-stop hook. If there are errors with pre/post hooks, they are
// logged but the subprocess is still run.
func HookCommand(ctx context.Context, cmd *exec.Cmd, preStart, postStop Hook) (int, error) {
	sigCh := make(chan os.Signal, 32)
	signal.Notify(sigCh)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	if preStart != nil {
		err := preStart(ctx, cmd)
		if err != nil {
			slog.Error("unable to run post stop hook", slog.Any("error", err))
		}
	}

	if postStop != nil {
		defer func() {
			err := postStop(ctx, cmd)
			if err != nil {
				slog.Error("unable to run post stop hook", slog.Any("error", err))
			}
		}()
	}

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("unable to start command: %w", err)
	}

	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGCHLD {
				continue
			}
			err := cmd.Process.Signal(sig)
			if err != nil {
				slog.Info("unable to send signal to child process", slog.Any("error", err), "signal", sig.String())
			}
		}
	}()

	err := cmd.Wait()
	// we will extract the exit code after and exit with the proper
	// code from cmd.ProcessState
	exitCode := 0
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return -1, fmt.Errorf("failed to run command: %w", err)
		}
		ws, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok && ws.Signaled() {
			exitCode = 128 + int(ws.Signal())
		} else {
			exitCode = exitErr.ExitCode()
		}

	}
	return exitCode, nil

}
