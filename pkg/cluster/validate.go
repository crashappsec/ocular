// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cluster

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// validateContext ensures that all resources that are required
// for scanning exist, and the [Context] can access them.
func validateContext(ctx context.Context, clusterCtx Context) error {
	// check configmap for profiles
	_, err := clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace).
		Get(ctx, config.State.Profiles.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to check profile config map: %w", err)
	}

	// check configmap for profiles
	_, err = clusterCtx.CS.CoreV1().Secrets(clusterCtx.Namespace).
		Get(ctx, config.State.Secrets.SecretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to check secret: %w", err)
	}

	_, err = clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace).
		Get(ctx, config.State.Downloaders.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to check downloader config map: %w", err)
	}

	_, err = clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace).
		Get(ctx, config.State.Crawlers.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to check crawler config map: %w", err)
	}

	_, err = clusterCtx.CS.CoreV1().ConfigMaps(clusterCtx.Namespace).
		Get(ctx, config.State.Uploaders.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to check uploader config map: %w", err)
	}

	return nil
}
