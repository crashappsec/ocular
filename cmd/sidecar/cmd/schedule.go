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
	"k8s.io/utils/ptr"

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
	pipelineFIFOPath := os.Getenv(v1beta1.EnvVarPipelineFIFO)
	searchFIFOPath := os.Getenv(v1beta1.EnvVarSearchFIFO)

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
	log.Info("creating FIFOs", "pipeline-fifo", pipelineFIFOPath, "search-fifo", searchFIFOPath)

	targets, crawlers := make(chan v1beta1.Target), make(chan v1beta1.ParameterizedObjectReference)

	if err = createFIFO(ctx, pipelineFIFOPath); err != nil {
		return fmt.Errorf("unable to create pipeline FIFO")
	}
	defer utils.RemoveAndLog(ctx, pipelineFIFOPath)

	if err = createFIFO(ctx, searchFIFOPath); err != nil {
		return fmt.Errorf("unable to create search FIFO")
	}
	defer utils.RemoveAndLog(ctx, searchFIFOPath)

	// crawlerCtx will have an event sent to the Done channel
	// when the crawler container exits for this search.
	crawlerCtx, crawlerCancel := context.WithCancel(ctx)

	log.Info("starting workers")
	wg := &sync.WaitGroup{}

	// crawler decoder and scheduler
	wg.Go(func() {
		fifoDecoder(crawlerCtx, crawlers, searchFIFOPath)
	})
	wg.Go(func() {
		for {
			crawler, ok := <-crawlers
			if !ok {
				log.Info("crawler channel closed")
				break
			}
			log.Info("scheduling search for crawler", "crawler", crawler)
			search := &v1beta1.Search{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: searchName + "-",
					Namespace:    namespace,
				},
				Spec: v1beta1.SearchSpec{
					Scheduler: v1beta1.SearchSchedulerSpec{
						IntervalSeconds: ptr.To(int32(sleepDuration)),
					},
					CrawlerRef: v1beta1.ParameterizedObjectReference{},
				},
			}
			crawler.DeepCopyInto(&search.Spec.CrawlerRef)
			template.Spec.DeepCopyInto(&search.Spec.Scheduler.PipelineTemplate.Spec)
			_, err = cs.ApiV1beta1().Searches(namespace).Create(ctx, search, metav1.CreateOptions{})
			if err != nil {
				log.Error(err, "unable to start pipeline for crawler", "crawler", crawler)
				continue
			}
			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}
		log.Info("search scheduler complete")
	})

	// pipeline decoder and scheduler
	wg.Go(func() {
		fifoDecoder(crawlerCtx, targets, pipelineFIFOPath)
	})

	wg.Go(func() {
		pipelineLabels := utils.MergeMaps(template.Labels, map[string]string{
			v1beta1.SearchLabelKey: searchName,
		})
		pipelineAnnotations := template.Annotations
		for {
			target, ok := <-targets
			if !ok {
				log.Info("target channel closed")
				break
			}
			log.Info("scheduling pipeline for target", "target", target)
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
		}
		log.Info("pipeline scheduler complete")
	})

	// await user crawler completion
	completePath := os.Getenv(v1beta1.EnvVarSidecarSchedulerCompletePath)
	for {
		select {
		case <-crawlerCtx.Done():
			log.Info("stopping scheduler")
			goto complete
		default:
			_, err := os.Stat(completePath)
			if err == nil {
				log.Info("complete path written, stopping scheduler")
				goto complete
			}
			time.Sleep(time.Second * 5)

		}
	}
complete:
	crawlerCancel()
	wg.Wait()
	return nil
}

func createFIFO(ctx context.Context, path string) error {
	log := logf.FromContext(ctx)

	if err := syscall.Mkfifo(path, 0622); err != nil {
		return fmt.Errorf("unable to create FIFO: %w", err)
	}

	if err := os.Chmod(path, 0622); err != nil {
		log.Error(err, "failed to change permissions of FIFO", "path", path)
	}
	return nil

}

func openFIFOReader(ctx context.Context, path string) (io.ReadCloser, error) {
	log := logf.FromContext(ctx)
	fifoReader, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
	if err != nil {
		log.Error(err, "fifo reader opening, error")
		return nil, fmt.Errorf("unable to open FIFO: %w", err)
	}

	return fifoReader, nil

}

func fifoDecoder[T any](ctx context.Context, c chan T, path string) {
	log := logf.FromContext(ctx, "fifo-path", path)
	var (
		decoder *json.Decoder
		rc      io.ReadCloser
		err     error
	)

	for {
		var t T
		if decoder == nil {
			rc, err = openFIFOReader(ctx, path)
			if err != nil {
				log.Error(err, "unable to open search fifo")
				continue
			}

			decoder = json.NewDecoder(rc)
		}
		err = decoder.Decode(&t)
		if errors.Is(err, io.EOF) {
			decoder = nil
			utils.CloseAndLog(ctx, rc, "closing fifo reader")
			time.Sleep(time.Second)
		} else if err != nil {
			log.Error(err, "error decoding from fifo")
			decoder = nil
			utils.CloseAndLog(ctx, rc, "closing search fifo reader")
			continue
		} else {
			log.Info("decoded from reader", "t", t)
			c <- t
			continue
		}
		select {
		case <-ctx.Done():
			log.Info("received signal, exiting decoder")
			close(c)
			return
		default:
		}
	}

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
