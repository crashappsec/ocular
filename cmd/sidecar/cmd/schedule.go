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

	pipelineFIFOReader, pipelineFIFOCleanup, err := openFIFOReader(ctx, pipelineFIFOPath)
	if err != nil {
		return err
	}

	defer pipelineFIFOCleanup()

	searchFIFOReader, searchFIFOCleanup, err := openFIFOReader(ctx, searchFIFOPath)
	if err != nil {
		return err
	}

	defer searchFIFOCleanup()

	targets, crawlers := make(chan v1beta1.Target), make(chan v1beta1.ParameterizedObjectReference)
	pipelineLabels := utils.MergeMaps(template.Labels, map[string]string{
		v1beta1.SearchLabelKey: searchName,
	})
	pipelineAnnotations := template.Annotations

	pipelineDecoder := json.NewDecoder(pipelineFIFOReader)
	searchDecoder := json.NewDecoder(searchFIFOReader)
	crawlerCtx, crawlerCancel := context.WithCancel(ctx)

	wg := &sync.WaitGroup{}

	wg.Go(func() {
		for {
			select {
			case crawler := <-crawlers:
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
				continue
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
			case <-crawlerCtx.Done():
				// crawlerCtx is done when the user crawler exits
				// and two channels are empty
				log.Info("pipeline scheduler exiting")
				return
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-crawlerCtx.Done():
				log.Info("received signal, exiting target decoder")
				return
			default:
				var target v1beta1.Target
				err = pipelineDecoder.Decode(&target)
				if err != nil && !errors.Is(err, io.EOF) {
					log.Error(err, "error decoding target from fifo")
					continue
				} else if err == nil {
					targets <- target
				}
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-crawlerCtx.Done():
				log.Info("received signal, exiting crawler decoder")
				return
			default:
				var crawler v1beta1.ParameterizedObjectReference
				err = searchDecoder.Decode(&crawler)
				if err != nil && !errors.Is(err, io.EOF) {
					log.Error(err, "error decoding target from fifo")
					continue
				} else if err == nil {
					crawlers <- crawler
				}
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping scheduler")
			default:
				_, err := StatCompletePath(ctx)
				if err != nil {
					log.Info("complete path written, stopping scheduler")
					crawlerCancel()
				}
				time.Sleep(time.Second * 5)
			}
		}
	})

	wg.Wait()
	return nil
}

func openFIFOReader(ctx context.Context, path string) (io.ReadCloser, func(), error) {
	log := logf.FromContext(ctx)
	if err := syscall.Mkfifo(path, 0622); err != nil {
		return nil, nil, fmt.Errorf("unable to create FIFO: %w", err)
	}

	if err := os.Chmod(path, 0622); err != nil {
		log.Error(err, "failed to change permissions of FIFO", "path", path)
	}

	fifoReader, err := os.OpenFile(path, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("unable to open FIFO: %w", err)
	}

	cleanup := func() {
		utils.CloseAndLog(ctx, fifoReader, "closing fifo reader")
		if err := os.Remove(path); err != nil {
			log.Error(err, "failed to cleanup pipeline FIFO", "path", path)
		}
	}
	return fifoReader, cleanup, nil

}

func StatPipelineFIFO(ctx context.Context) (os.FileInfo, error) {
	path := os.Getenv(v1beta1.EnvVarPipelineFIFO)
	return os.Stat(path)
}

func StatSearchFIFO(ctx context.Context) (os.FileInfo, error) {
	path := os.Getenv(v1beta1.EnvVarSearchFIFO)
	return os.Stat(path)
}

func StatCompletePath(ctx context.Context) (os.FileInfo, error) {
	path := os.Getenv(v1beta1.EnvVarSidecarSchedulerCompletePath)
	return os.Stat(path)
}

func CreateCompleteFile(ctx context.Context) error {
	path := os.Getenv(v1beta1.EnvVarSidecarSchedulerCompletePath)
	_, err := os.Create(path)
	return err
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
