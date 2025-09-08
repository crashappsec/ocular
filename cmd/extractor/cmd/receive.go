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

	v1beta1 "github.com/crashappsec/ocular/api/v1beta1"
	"github.com/crashappsec/ocular/internal/utils"
	"go.uber.org/zap"
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
		logger.Info("received upload request", zap.String("path", path))
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

		mutex.Lock()
		defer mutex.Unlock()
		written, exists := downloadedFiles[file]
		if !exists {
			logger.Info("file is not in the list of files to download", zap.String("path", file))
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if written {
			logger.Info("file is already downloaded", zap.String("path", file))
			w.WriteHeader(http.StatusConflict)
			return
		}

		defer wg.Done()
		defer utils.CloseAndLog(ctx, r.Body, "closing upload request body")
		dst, err := os.Create(filepath.Clean(file))
		if err != nil {
			logger.Error(err, "failed to create file", "path", file)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer utils.CloseAndLog(ctx, dst, "closing uploaded file writer")
		if _, err = io.Copy(dst, r.Body); err != nil && !errors.Is(err, io.EOF) {
			logger.Error(err, "failed to write file", "path", file)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		logger.Info("file downloaded", "path", file)
		w.WriteHeader(http.StatusOK)
		downloadedFiles[file] = true
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
		wg.Wait()
		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error(err, "Error shutting down server from completion")
		}
	}()
	logger.Info("starting server", "address", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
