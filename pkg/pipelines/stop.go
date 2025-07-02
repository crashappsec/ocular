// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package pipelines

import (
	"context"

	"github.com/crashappsec/ocular/pkg/runtime"
	"github.com/crashappsec/ocular/pkg/schemas"
	"go.uber.org/zap"
	typedBatchV1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

func Stop(
	ctx context.Context,
	jobInterface typedBatchV1.JobInterface,
	id schemas.ExecutionID,
) error {
	scanStopErr := runtime.StopJob(
		ctx, jobInterface, id, scanJobType)
	uploadStopErr := runtime.StopJob(
		ctx, jobInterface, id, uploadJobType)
	if uploadStopErr != nil {
		// we just log here since this job may not exist
		zap.L().Error("Failed to stop upload job", zap.Error(uploadStopErr))
	}
	return scanStopErr
}
