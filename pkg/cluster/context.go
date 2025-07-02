// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cluster

//go:generate mockgen -destination ../../internal/unittest/mocks/internal/cluster/context.go -package=cluster -typed . ContextManager

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type Context struct {
	Name      string
	Namespace string
	CS        *kubernetes.Clientset
}

type ContextManager interface {
	HasContext(string) bool
	GetContext(string) (Context, bool)
	DefaultContext() (Context, bool)
}

var _ ContextManager = (*contextManager)(nil)

type contextManager struct {
	ctxs           map[string]Context
	defaultContext *Context
}

type ContextManagerOpts struct {
	DisableInClusterContext bool
	CheckValidity           bool
	InClusterNamespace      string
}

func NewContextManager(
	ctx context.Context,
	kubeConfig io.Reader,
	opts ContextManagerOpts,
) (ContextManager, error) {
	ctxManager := &contextManager{}

	if kubeConfig != nil {
		ctxs, currentCtx, err := ParseKubeConfigClients(kubeConfig)
		if err != nil {
			return nil, err
		}
		ctxManager.ctxs = ctxs

		if defaultCtx, ok := ctxs[currentCtx]; ok {
			ctxManager.defaultContext = &defaultCtx
		}
	}

	if !opts.DisableInClusterContext {
		clusterCtx, err := NewInClusterContext(opts.InClusterNamespace)
		if err != nil {
			return nil, err
		}
		ctxManager.defaultContext = &clusterCtx
	}

	if opts.CheckValidity {
		for name, clusterCtx := range ctxManager.ctxs {
			// TODO: should fatally error if we can't view them
			if err := validateContext(ctx, clusterCtx); err != nil {
				zap.L().
					Error("removing invalid context", zap.String("context", name), zap.Error(err))
				delete(ctxManager.ctxs, name)
			}
		}
	}

	if ctxManager.defaultContext == nil && len(ctxManager.ctxs) == 0 {
		return nil, errors.New("no valid contexts found")
	}

	zap.L().Info(fmt.Sprintf("loaded %d contexts", len(ctxManager.ctxs)),
		zap.Int("contexts", len(ctxManager.ctxs)),
		zap.Bool("default_enabled", ctxManager.defaultContext != nil))

	return ctxManager, nil
}

func (c contextManager) HasContext(s string) bool {
	if c.ctxs == nil {
		return false
	}
	_, ok := c.ctxs[s]
	return ok
}

func (c contextManager) GetContext(s string) (Context, bool) {
	if s == "in-cluster" {
		// TODO(bryce): clean this special case up
		return c.DefaultContext()
	}

	if c.ctxs == nil {
		return Context{}, false
	}
	val, ok := c.ctxs[s]
	return val, ok
}

func (c contextManager) DefaultContext() (Context, bool) {
	if c.defaultContext == nil {
		return Context{}, false
	}
	return *c.defaultContext, true
}
