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
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "github.com/crashappsec/ocular/api/v1"
	"github.com/crashappsec/ocular/internal/utils"
	"go.uber.org/zap"
)

func Receive(ctx context.Context, files []string) error {
	port := os.Getenv(v1.EnvVarExtractorPort)
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
		zap.L().Info("received upload request", zap.String("path", path))
		file, err := url.PathUnescape(path)
		if err != nil {
			zap.L().Warn("file name is not valid", zap.String("path", path))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !filepath.IsAbs(file) {
			w.WriteHeader(http.StatusBadRequest)
			zap.L().Warn("file is not absolute path", zap.String("path", file))
			return
		}

		mutex.Lock()
		written, exists := downloadedFiles[file]
		mutex.Unlock()
		if !exists {
			zap.L().Warn("file is not in the list of files to download", zap.String("path", file))
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if written {
			zap.L().Warn("file is already downloaded", zap.String("path", file))
			w.WriteHeader(http.StatusConflict)
			return
		}

		defer wg.Done()
		utils.CloseAndLog(ctx, r.Body, "closing upload request body")
		dst, err := os.Create(filepath.Clean(file))
		if err != nil {
			zap.L().Error("failed to create file", zap.String("path", file), zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		utils.CloseAndLog(ctx, dst, "closing uploaded file writer")
		if _, err = io.Copy(dst, r.Body); err != nil && !errors.Is(err, io.EOF) {
			zap.L().Error("failed to write file", zap.String("path", file), zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		zap.L().Info("file downloaded", zap.String("path", file))
		w.WriteHeader(http.StatusOK)
		mutex.Lock()
		defer mutex.Unlock()
		downloadedFiles[file] = true
	})

	// start server
	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		zap.L().Error("Received /fail request")
		w.WriteHeader(http.StatusCreated)
		err := srv.Shutdown(ctx)
		if err != nil {
			zap.L().Error("Error shutting down server from fail request", zap.Error(err))
		}
	})
	go func() {
		wg.Wait()
		err := srv.Shutdown(ctx)
		if err != nil {
			zap.L().Error("Error shutting down server from completion", zap.Error(err))
		}
	}()
	zap.L().Info("Starting server", zap.String("address", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
