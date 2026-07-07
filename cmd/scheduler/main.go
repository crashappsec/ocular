// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Scheduler is a binary that is loaded into a shared
// volume mount and wraps crawler executions to provide
// an interface for crawler to spawn new pipelines/searches.
// It will create a FIFO for each and then spawn the user process.
// If the user process writes the FIFO, the respective resource
// will be created. Once the user process exits and all resources
// have been scheduled, the program will exit.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/process"
	"github.com/crashappsec/ocular/internal/utils"
	"github.com/crashappsec/ocular/pkg/generated/clientset"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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
	slog.SetDefault(l)

	l.Info("starting ocular scheduler")
	if len(os.Args) < 2 {
		l.Error("no command specified for scheduler")
		fmt.Println("Usage: scheduler <command> [user command...]")
		os.Exit(1)
	}

	var (
		command = os.Args[1]
		userCmd = os.Args[2:]
	)

	ctx, awaitSigterm := process.CancelContextSigterm(ctx)
	go awaitSigterm()

	l = l.With(slog.Any("userCmd", userCmd), slog.String("command", command))
	l.Info("starting scheduler command " + command)
	switch command {
	case "init":
		execPath, err := os.Executable()
		if err != nil {
			l.Error("unable to determine executable path", slog.Any("error", err))
			os.Exit(1)
		}
		runtimePath := os.Getenv(v1beta1.EnvVarSchedulerPath)
		err = process.CopyFile(ctx, execPath, runtimePath)
		if err != nil {
			l.Error("failed to copy executable", slog.Any("error", err))
			os.Exit(1)
		}
		err = os.Chmod(runtimePath, 0o755)
		if err != nil {
			l.Error("unable to change permissions of executable", slog.Any("error", err))
		}
	case "crawler":
		cmd, err := process.BuildUserCommand(ctx, userCmd)
		if err != nil {
			l.Error("unable to parse user command", slog.Any("error", err))
			os.Exit(1)
		}

		awaitSchedulerHook, err := Schedule(ctx)
		if err != nil {
			l.Error("unable to start scheduler", slog.Any("error", err))
			os.Exit(1)
		}
		exitCode, err := process.HookCommand(ctx, cmd, nil, awaitSchedulerHook)
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

func Schedule(ctx context.Context) (process.Hook, error) {
	slog.Info("beginning pipeline scheduler")

	cs, err := parseKubernetesClientset(ctx)
	if err != nil {
		slog.Error("unable to create kubernetes clientset, disabling scheduler", slog.Any("error", err))
		return nil, fmt.Errorf("unable to create kubernetes clientset")
	}

	searchName := os.Getenv(v1beta1.EnvVarSearchName)
	namespace := os.Getenv(v1beta1.EnvVarNamespaceName)
	pipelineFIFOPath := os.Getenv(v1beta1.EnvVarPipelineFIFO)
	searchFIFOPath := os.Getenv(v1beta1.EnvVarSearchFIFO)

	templateFilePath := os.Getenv(v1beta1.EnvVarPipelineTemplatePath)
	sleepDuration, err := strconv.Atoi(os.Getenv(v1beta1.EnvVarPipelineSchedulerIntervalSeconds))
	if err != nil {
		slog.Error("unable to parse sleep seconds, defaulting to 60", slog.Any("error", err))
		sleepDuration = 60
	} else if sleepDuration < 0 {
		slog.Info("negative sleep amount given, defaulting to 60")
		sleepDuration = 60
	}

	templateFile, err := os.Open(templateFilePath)
	if err != nil {
		slog.Error("unable to open pipeline template file", slog.String("file", templateFilePath), slog.Any("error", err))
		return nil, fmt.Errorf("unable to open pipeline template file")
	}

	var template v1beta1.PipelineTemplate
	if err = json.NewDecoder(templateFile).Decode(&template); err != nil {
		slog.Error("unable to decode pipeline template", slog.String("file", templateFilePath), slog.Any("error", err))
		return nil, fmt.Errorf("unable to decode pipeline template")
	}
	slog.Info("creating FIFOs", slog.String("pipeline-fifo", pipelineFIFOPath), slog.String("search-fifo", searchFIFOPath))

	targets, crawlers := make(chan v1beta1.Target), make(chan v1beta1.ParameterizedLocalObjectReference)

	if err = createFIFO(ctx, pipelineFIFOPath); err != nil {
		return nil, fmt.Errorf("unable to create pipeline FIFO")
	}

	if err = createFIFO(ctx, searchFIFOPath); err != nil {
		utils.RemoveAndLog(ctx, pipelineFIFOPath)
		return nil, fmt.Errorf("unable to create search FIFO")
	}

	// crawlerCtx will have an event sent to the Done channel
	// when the crawler container exits for this search.
	crawlerCtx, crawlerCancel := context.WithCancel(ctx)

	scheduledByLabels := make(map[string]string)
	scheduledByLabels[v1beta1.ScheduledByLabelKey] = searchName

	ownerRef := metav1.OwnerReference{
		UID:        types.UID(os.Getenv(v1beta1.EnvVarSchedulerParentUID)),
		Kind:       "Search",
		APIVersion: v1beta1.GroupVersion.String(),
		Name:       searchName,
		Controller: new(true),
	}

	slog.Info("starting workers")
	wg := &sync.WaitGroup{}

	// crawler decoder and scheduler
	wg.Go(func() {
		fifoDecoder(crawlerCtx, crawlers, searchFIFOPath)
	})
	wg.Go(func() {
		var ttlSeconds *int32
		if ttlEnv := os.Getenv(v1beta1.EnvVarSchedulerSearchTTL); ttlEnv != "" {
			ttl, err := strconv.Atoi(ttlEnv)
			if err == nil && ttl != 0 {
				ttlSeconds = new(int32(ttl))
			}
		}
		serviceAccount := os.Getenv(v1beta1.EnvVarSchedulerServiceAccount)
		for {
			crawler, ok := <-crawlers
			if !ok {
				slog.Info("crawler channel closed")
				break
			}
			slog.Info("scheduling search for crawler", slog.Any("crawler", crawler))
			search := &v1beta1.Search{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: searchName + "-",
					Namespace:    namespace,
					Labels:       scheduledByLabels,
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef.DeepCopy(),
					},
				},
				Spec: v1beta1.SearchSpec{
					TTLSecondsAfterFinished: ttlSeconds,
					ServiceAccountName:      serviceAccount,
					Scheduler: v1beta1.SearchSchedulerSpec{
						IntervalSeconds: new(int32(sleepDuration)),
					},
					CrawlerRef: v1beta1.ParameterizedLocalObjectReference{},
				},
			}
			crawler.DeepCopyInto(&search.Spec.CrawlerRef)
			template.Spec.DeepCopyInto(&search.Spec.Scheduler.PipelineTemplate.Spec)
			slog.Info("starting search", slog.Any("search", search))
			scheduledSearch, err := cs.ApiV1beta1().Searches(namespace).Create(ctx, search, metav1.CreateOptions{})
			if err != nil {
				slog.Error("unable to start pipeline for crawler", slog.Any("crawler", crawler), slog.Any("error", err))
				continue
			}
			slog.Info("search created", "search", scheduledSearch.Name)

			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}
		slog.Info("search scheduler complete")
	})

	wg.Go(func() {
		// pipeline decoder and scheduler
		fifoDecoder(crawlerCtx, targets, pipelineFIFOPath)
	})

	wg.Go(func() {
		pipelineLabels := utils.MergeMaps(template.Labels, scheduledByLabels)
		pipelineAnnotations := template.Annotations
		for {
			target, ok := <-targets
			if !ok {
				slog.Info("target channel closed")
				break
			}

			slog.Info("scheduling pipeline for target", "target", target)
			pipeline := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: searchName + "-",
					Namespace:    namespace,
					Annotations:  maps.Clone(pipelineAnnotations),
					Labels:       maps.Clone(pipelineLabels),
					OwnerReferences: []metav1.OwnerReference{
						*ownerRef.DeepCopy(),
					},
				},
			}

			template.Spec.DeepCopyInto(&pipeline.Spec)

			target.DeepCopyInto(&pipeline.Spec.Target)
			scheduledPipeline, err := cs.ApiV1beta1().Pipelines(namespace).Create(ctx, pipeline, metav1.CreateOptions{})
			if err != nil {
				slog.Error("unable to start pipeline for target", slog.Any("target", target), slog.Any("error", err))
				continue
			}
			slog.Info("pipeline created", "pipeline", scheduledPipeline.Name)
			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}
		slog.Info("pipeline scheduler complete")
	})

	return func(ctx context.Context, _ *exec.Cmd) error {
		crawlerCancel()
		wg.Wait()
		utils.RemoveAndLog(ctx, pipelineFIFOPath)
		utils.RemoveAndLog(ctx, searchFIFOPath)
		return nil
	}, nil
}

func createFIFO(_ context.Context, path string) error {
	if err := syscall.Mkfifo(path, 0622); err != nil {
		return fmt.Errorf("unable to create FIFO: %w", err)
	}

	if err := os.Chmod(path, 0622); err != nil {
		slog.Error("failed to change permissions of FIFO", slog.String("path", path), slog.Any("error", err))
	}
	return nil

}

func openFIFOReader(_ context.Context, path string) (io.ReadCloser, error) {
	fifoReader, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
	if err != nil {
		slog.Error("fifo reader opening, error", slog.Any("error", err))
		return nil, fmt.Errorf("unable to open FIFO: %w", err)
	}

	return fifoReader, nil

}

func fifoDecoder[T any](ctx context.Context, c chan T, path string) {
	var (
		decoder *json.Decoder
		rc      io.ReadCloser
		err     error
		l       = slog.With(slog.String("fifo", path))
	)

	for {
		var t T
		if decoder == nil {
			rc, err = openFIFOReader(ctx, path)
			if err != nil {
				l.Error("unable to open fifo", slog.Any("error", err))
				time.Sleep(time.Second)
				continue
			}
			decoder = json.NewDecoder(rc)
		}
		l.Info("decoding from FIFO")
		err = decoder.Decode(&t)
		if err != nil {
			decoder = nil
			if !errors.Is(err, io.EOF) {
				l.Error("error from decoder", slog.Any("error", err))
			}
			process.CloseAndLog(ctx, rc)
			time.Sleep(time.Second)
		} else {
			l.Info("decoded from reader", slog.Any("t", t))
			c <- t
			continue
		}
		select {
		case <-ctx.Done():
			l.Info("received signal, exiting decoder")
			close(c)
			return
		default:
		}
	}

}

func parseKubernetesClientset(_ context.Context) (*clientset.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)

	if config, err = rest.InClusterConfig(); err != nil {
		slog.Info("in-cluster configuration was unable to be parsed, trying kubeconfig")
		home := homedir.HomeDir()
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			slog.Info("unable to build kubernetes config from flags, trying kubeconfig")
			return nil, fmt.Errorf("unable to parse in-cluster config and kubeconfig")
		}
	}

	cs, err := clientset.NewForConfig(config)
	return cs, err
}
