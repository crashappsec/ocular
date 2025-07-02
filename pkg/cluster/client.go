// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package cluster provides function to interact with Kubernetes clusters.
package cluster

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewInClusterContext creates a new [Context] for the in-cluster configuration.
// An "in-cluster" configuration means that the API server will assume its running
// inside a pod in a Kubernetes cluster and will configure the REST client from
// the pod's service account token and CA certificate.
func NewInClusterContext(namespace string) (Context, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return Context{}, err
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return Context{}, err
	}

	if namespace == "" {
		namespace = "default"
	}

	return Context{
		Namespace: namespace,
		Name:      "in-cluster",
		CS:        k8sClient,
	}, nil
}

// ParseKubeConfigClients parses the kubeconfig file and returns a map of context names
// with their respective [Context]. It also returns the current context name.
func ParseKubeConfigClients(value io.Reader) (map[string]Context, string, error) {
	contents, err := io.ReadAll(value)
	if err != nil {
		return nil, "", err
	}

	kubeconfig, err := clientcmd.Load(contents)
	if err != nil {
		return nil, "", err
	}

	var (
		ctxs     = make(map[string]Context)
		multierr *multierror.Error
	)

	for contextName := range kubeconfig.Contexts {
		clientConfig := clientcmd.NewNonInteractiveClientConfig(
			*kubeconfig,
			contextName,
			&clientcmd.ConfigOverrides{},
			nil,
		)

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			multierr = multierror.Append(
				multierr,
				fmt.Errorf("failed to parse rest config from context '%s': %w", contextName, err),
			)
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			multierr = multierror.Append(
				multierr,
				fmt.Errorf(
					"failed to initialize client config from context '%s': %w",
					contextName,
					err,
				),
			)
		}

		namespace, _, err := clientConfig.Namespace()
		if err != nil {
			multierr = multierror.Append(
				multierr,
				fmt.Errorf("failed to parse namespace from context '%s': %w", contextName, err),
			)
		}

		ctxs[contextName] = Context{
			Name:      contextName,
			CS:        clientset,
			Namespace: namespace,
		}
	}

	return ctxs, kubeconfig.CurrentContext, multierr.ErrorOrNil()
}
