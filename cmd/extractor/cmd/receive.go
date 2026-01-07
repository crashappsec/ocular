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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/utils"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func Receive(ctx context.Context, files []string) error {
	logger := logf.FromContext(ctx)
	port := os.Getenv(v1beta1.EnvVarExtractorPort)
	var (
		mux             = http.NewServeMux()
		downloadedFiles = map[string]bool{}
		mutex           = &sync.Mutex{}
		wg              = &sync.WaitGroup{}
	)

	for _, file := range files {
		downloadedFiles[file] = false
		wg.Add(1)
	}
	mux.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		// check if file is already downloaded
		path := strings.TrimPrefix(r.URL.Path, "/upload/")
		logger.Info("received upload request", "path", path)
		file, err := url.PathUnescape(path)
		if err != nil {
			logger.Info("file name is not valid", "path", path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !filepath.IsAbs(file) {
			w.WriteHeader(http.StatusBadRequest)
			logger.Info("file is not absolute path", "path", file)
			return
		}

		isResultFile := strings.HasPrefix(file, v1beta1.PipelineResultsDirectory)
		isMetadataFile := strings.HasPrefix(file, v1beta1.PipelineMetadataDirectory)

		if !isResultFile && !isMetadataFile {
			w.WriteHeader(http.StatusBadRequest)
			logger.Info("file is not located in whitelisted directories", "path", file)
			return
		}

		mutex.Lock()
		written, exists := downloadedFiles[file]
		defer mutex.Unlock()
		if !exists {
			logger.Info("file is not in the list of files to download", "path", file)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if written {
			logger.Info("file is already downloaded", "path", file)
			w.WriteHeader(http.StatusConflict)
			return
		}

		if r.ContentLength > 0 {
			defer utils.CloseAndLog(ctx, r.Body, "closing upload request body")
			dst, err := os.Create(filepath.Clean(file))
			if err != nil {
				logger.Error(err, "failed to create file", "path", file)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer utils.CloseAndLog(ctx, dst, "closing uploaded file writer")
			_, err = io.Copy(dst, r.Body)
			if err != nil && !errors.Is(err, io.EOF) {
				logger.Error(err, "failed to write file", "path", file)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			logger.Info("file given with zero content length, assuming missing file and will not create", "path", file)
		}
		wg.Done()
		logger.Info("file downloaded", "path", file)
		downloadedFiles[file] = true
		w.WriteHeader(http.StatusOK)
	})

	// start server
	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		logger.Error(fmt.Errorf("received /fail request"), "shutting down, received /fail request")
		w.WriteHeader(http.StatusCreated)
		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error(err, "Error shutting down server from fail request")
		}
	})
	go func() {
		logger.Info("starting server", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(err, "server error")
			panic(err)
		}
	}()
	logger.Info("awaiting file downloads", "count", len(files))
	wg.Wait()
	logger.Info("all files downloaded, shutting down server")
	err := srv.Shutdown(ctx)
	if err != nil {
		logger.Error(err, "Error shutting down server from completion")
	}
	return nil
}
