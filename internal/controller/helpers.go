// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package controller

import (
	"context"
	"fmt"

	"github.com/crashappsec/ocular/internal/resources"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// updateStatus will update the status of any [sigs.k8s.io/controller-runtime/pkg/client.Object]
// using the provided controller-runtime client. It will log any errors encountered during the update process
func updateStatus(ctx context.Context, c client.Client, obj client.Object, errorMsgKeysAndValues ...any) error {
	l := logf.FromContext(ctx)
	if updateErr := c.Status().Update(ctx, obj); updateErr != nil {
		l.Error(updateErr, fmt.Sprintf("failed to update status of %s", obj.GetName()), errorMsgKeysAndValues...)
		return updateErr
	}
	return nil
}

func generateBaseContainerOptions(envVars []corev1.EnvVar) []resources.ContainerOption {
	return []resources.ContainerOption{
		resources.ContainerWithAdditionalEnvVars(envVars...),
		resources.ContainerWithPodSecurityStandardRestricted(),
	}
}
