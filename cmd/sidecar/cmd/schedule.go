// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/utils"
	"github.com/crashappsec/ocular/pkg/generated/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func Schedule(ctx context.Context) error {
	log := logf.FromContext(ctx)
	log.Info("beginning pipeline scheduler")

	cs, err := parseKubernetesClientset(ctx)
	if err != nil {
		log.Error(err, "unable to create kubernetes clientset, disabling scheduler")
		return fmt.Errorf("unable to create kubernetes clientset")
	}

	searchName := os.Getenv(v1beta1.EnvVarSearchName)
	namespace := os.Getenv(v1beta1.EnvVarNamespaceName)
	fifoPath := os.Getenv(v1beta1.EnvVarPipelineFifo)

	templateFilePath := os.Getenv(v1beta1.EnvVarPipelineTemplatePath)
	sleepDuration, err := strconv.Atoi(os.Getenv(v1beta1.EnvVarPipelineSchedulerIntervalSeconds))
	if err != nil {
		log.Error(err, "unable to parse sleep seconds, defaulting to 60")
		sleepDuration = 60
	} else if sleepDuration < 0 {
		log.Info("negative sleep amount given, defaulting to 60")
		sleepDuration = 60
	}

	templateFile, err := os.Open(templateFilePath)
	if err != nil {
		log.Error(err, "unable to open pipeline template file", "file", templateFile)
		return fmt.Errorf("unable to open pipeline template file")
	}

	var template v1beta1.PipelineTemplate
	if err = json.NewDecoder(templateFile).Decode(&template); err != nil {
		log.Error(err, "unable to decode pipeline template", "file", templateFile)
		return fmt.Errorf("unable to decode pipeline template")
	}

	if err = syscall.Mkfifo(fifoPath, 0622); err != nil {
		log.Error(err, "unable to create FIFO", "path", fifoPath)
		return fmt.Errorf("unable to create FIFO: %w", err)
	}

	defer func() {
		if err = os.Remove(fifoPath); err != nil {
			log.Error(err, "failed to cleanup FIFO", "path", fifoPath)
		}
	}()

	if err = os.Chmod(fifoPath, 0622); err != nil {
		log.Error(err, "failed to change permissions of FIFO", "path", fifoPath)
		return fmt.Errorf("unable to change permissions of FIFO: %w", err)
	}

	fifoReader, err := os.OpenFile(fifoPath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Error(err, "unable to create FIFO %s", fifoPath)
		return fmt.Errorf("unable to create FIFO: %w", err)
	}

	defer utils.CloseAndLog(ctx, fifoReader, "step", "close fifo reader")

	targets := make(chan v1beta1.Target)
	complete := make(chan struct{})
	pipelineLabels := utils.MergeMaps(template.Labels, map[string]string{
		v1beta1.SearchLabelKey: searchName,
	})
	pipelineAnnotations := template.Annotations

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case target := <-targets:
				pipeline := &v1beta1.Pipeline{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: searchName + "-",
						Namespace:    namespace,
						Annotations:  maps.Clone(pipelineAnnotations),
						Labels:       maps.Clone(pipelineLabels),
					},
				}

				template.Spec.DeepCopyInto(&pipeline.Spec)

				target.DeepCopyInto(&pipeline.Spec.Target)
				_, err = cs.ApiV1beta1().Pipelines(namespace).Create(ctx, pipeline, metav1.CreateOptions{})
				if err != nil {
					log.Error(err, "unable to start pipeline for target", "target", target)
					continue
				}
				time.Sleep(time.Duration(sleepDuration) * time.Second)
				continue
			case <-complete:
				err := os.Remove(fifoPath)
				if err != nil {
					log.Error(err, "unable to remove fifo")
				}
				return
			case <-ctx.Done():
				log.Info("pipeline scheduler exiting")
				return
			}
		}
	})

	decoder := json.NewDecoder(fifoReader)
	eof := false

	for !eof {
		select {
		case <-ctx.Done():
			log.Info("received signal, exiting pipeline scheduler")
			return nil
		default:
			var target v1beta1.Target
			err = decoder.Decode(&target)
			if errors.Is(err, io.EOF) {
				// signal for the other pod is done, so we should remove FIFO
				complete <- struct{}{}
				eof = true
				break
			} else if err != nil {
				log.Error(err, "error decoding target from fifo")
				continue
			}
			targets <- target
		}

	}

	wg.Wait()
	return nil

}

func StatFIFO(ctx context.Context) (os.FileInfo, error) {
	path := os.Getenv(v1beta1.EnvVarPipelineFifo)
	return os.Stat(path)
}

func parseKubernetesClientset(ctx context.Context) (*clientset.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)
	l := logf.FromContext(ctx)

	if config, err = rest.InClusterConfig(); err != nil {
		l.Info("in-cluster configuration was unable to be parsed, trying kubeconfig")
		home := homedir.HomeDir()
		kubeconfigPath := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			l.Info("unable to build kubernetes config from flags, trying kubeconfig")
			return nil, fmt.Errorf("unable to parse in-cluster config and kubeconfig")
		}
	}

	cs, err := clientset.NewForConfig(config)
	return cs, err
}
