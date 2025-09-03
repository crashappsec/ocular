// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	v1 "github.com/crashappsec/ocular/api/v1"
	"github.com/crashappsec/ocular/internal/utils"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Extract(ctx context.Context, files []string) error {
	l := log.FromContext(ctx)
	uploaderURL := os.Getenv(v1.EnvVarExtractorHost)
	err := uploadFiles(ctx, uploaderURL, files)
	if err != nil {
		l.Error(err, "error uploading files, failing receiver")
		if failErr := fail(ctx, uploaderURL); failErr != nil {
			l.Error(failErr, "failed to notify receiver of failure")
		}
	}
	return err
}

func uploadFiles(ctx context.Context, uploaderURL string, files []string) error {
	var (
		wg   = &sync.WaitGroup{}
		merr *multierror.Error
	)
	for _, file := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src, err := os.Open(filepath.Clean(file))
			if err != nil {
				merr = multierror.Append(merr, err)
				return
			}
			defer utils.CloseAndLog(ctx, src, "closing source file", "file", file)

			u := fmt.Sprintf("%s/upload/%s", uploaderURL, url.PathEscape(file))
			zap.L().Debug("Uploading file", zap.String("file", file), zap.String("url", u))

			req, err := http.NewRequest(http.MethodPut, u, src)
			if err != nil {
				merr = multierror.Append(merr, err)
				return
			}
			// Let the OS fill Content-Length automatically if possible:
			info, _ := src.Stat()
			req.ContentLength = info.Size()

			retries := 0
			for {
				resp, err := http.DefaultClient.Do(req)
				if err != nil || resp.StatusCode >= http.StatusInternalServerError {
					zap.L().Error("Error uploading file",
						zap.String("file", file),
						zap.String("url", u),
						zap.Error(err),
						zap.Int("retries", retries))
					if retries > 5 {
						merr = multierror.Append(merr, err)
						return
					}
					retries++
					zap.L().
						Debug("Retrying upload", zap.String("file", file), zap.Int("retries", retries))
					time.Sleep(time.Duration(retries) * time.Second)
					continue
				}

				utils.CloseAndLog(ctx, resp.Body, "closing upload response body")
				break
			}
			zap.L().Info("Uploaded file", zap.String("file", file))
		}()
	}

	wg.Wait()
	return merr.ErrorOrNil()
}

func fail(ctx context.Context, uploaderURL string) error {
	u := fmt.Sprintf("%s/fail", uploaderURL)

	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	utils.CloseAndLog(ctx, resp.Body, "closing fail upload response body")
	return nil
}
