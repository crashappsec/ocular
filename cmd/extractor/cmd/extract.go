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

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/utils"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Extract(ctx context.Context, files []string) error {
	l := log.FromContext(ctx)
	l.Info("beginning file extraction", "file_count", len(files))
	uploaderURL := os.Getenv(v1beta1.EnvVarExtractorHost)
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
		wg     = &sync.WaitGroup{}
		merr   *multierror.Error
		logger = log.FromContext(ctx)
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
			logger.Info("uploading file", "file", file, "url", u)

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
					logger.Error(err, "Error uploading file",
						"file", file, "url", u, "retries", retries)
					if retries > 5 {
						merr = multierror.Append(merr, err)
						return
					}
					retries++
					logger.
						Info("Retrying upload", "file", file, "retries", retries)
					time.Sleep(time.Duration(retries) * time.Second)
					continue
				}

				utils.CloseAndLog(ctx, resp.Body, "closing upload response body")
				break
			}
			logger.Info("Uploaded file", "file", file)
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
