// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Entrypoint for the API server.
// The API server is responsible for providing a RESTful API for the Ocular engine.
// It will store both the user configurations of profiles, uploaders, downloaders and
// crawlers, as well as orchestrate the execution of pipelines and searches.
package main

import (
	"context"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/api"
	"github.com/crashappsec/ocular/pkg/cluster"
	"go.uber.org/zap"
)

func init() {
	config.Init()
}

func main() {
	ctx := context.Background()
	l := zap.L()
	l.Info("starting api server")

	kubeConfig := config.ReadKubeConfig()

	l.Debug("config file loaded", zap.Any("config_file", config.State))
	l.Debug("loading context manager")
	ctxManager, err := cluster.NewContextManager(ctx, kubeConfig, cluster.ContextManagerOpts{
		DisableInClusterContext: !config.State.ClusterAccess.ServiceAccount.Enabled,
		InClusterNamespace:      config.State.ClusterAccess.ServiceAccount.Namespace,
		CheckValidity:           config.State.ClusterAccess.CheckValidity,
	})
	if err != nil {
		l.With(zap.Error(err)).Fatal("unable to configure context manager")
	}

	l.Debug("initializing api")
	engine, err := api.InitializeEngine(ctxManager)
	if err != nil {
		l.With(zap.Error(err)).Fatal("unable to initialize api router")
	}

	err = api.RunEngine(engine)
	if err != nil {
		l.With(zap.Error(err)).Fatal("http server exited with error")
	}
}
