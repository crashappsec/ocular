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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type filelocks map[string]*sync.Mutex

func (f filelocks) getLock(fpath string) (*sync.Mutex, bool) {
	lock, exists := f[fpath]
	return lock, exists
}

func authorizeUpload(ctx context.Context, v *jwtValidator, bearerToken, aud, ns string) error {
	claims, err := v.validateToken(ctx, bearerToken)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	expected := jwt.Expected{
		AnyAudience: jwt.Audience{aud},
		Time:        time.Now(),
	}
	if err := claims.Validate(expected); err != nil {
		return fmt.Errorf("failed audience in claims: %w", err)
	}

	if claims.Kubernetes.Namespace != ns {
		return fmt.Errorf("claims do not belong to expected namespace")
	}
	return nil
}

func uploadHandler(v *jwtValidator, wg *sync.WaitGroup, locks filelocks) func(http.ResponseWriter, *http.Request) {

	expectedAudience := os.Getenv(v1beta1.EnvVarPodName)
	expectedNamespace := os.Getenv(v1beta1.EnvVarNamespaceName)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logf.FromContext(ctx)

		bearerHeader := r.Header.Get("Authorization")
		if bearerHeader == "" || !strings.HasPrefix(bearerHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		err := authorizeUpload(ctx, v, strings.TrimPrefix(bearerHeader, "Bearer "), expectedAudience, expectedNamespace)
		if err != nil {
			logger.Error(err, "unable to authorize request", "request-path", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/upload/")
		logger.Info("received upload request", "path", path)
		fname, err := url.PathUnescape(path)
		if err != nil {
			logger.Info("file name is not valid", "path", path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		file := filepath.Clean(fname)

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

		filelock, exists := locks.getLock(file)
		if !exists {
			logger.Info("file is not in the list of files to download", "path", file)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		filelock.Lock()
		defer filelock.Unlock()

		_, err = os.Stat(file)
		if err == nil {
			logger.Info("file is already downloaded", "path", file)
			w.WriteHeader(http.StatusConflict)
			return
		} else if !os.IsNotExist(err) {
			logger.Error(err, "failed to stat file")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.ContentLength > 0 {
			defer utils.CloseAndLog(ctx, r.Body, "closing upload request body")
			dst, err := os.Create(file)
			if err != nil {
				logger.Error(err, "failed to create file", "path", file)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer utils.CloseAndLog(ctx, dst, "closing uploaded file writer")
			n, err := io.Copy(dst, r.Body)
			if err != nil && !errors.Is(err, io.EOF) {
				logger.Error(err, "failed to write file", "path", file, "bytes", n)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			logger.Info("successfully downloaded content", "path", file, "bytes", n)
		} else {
			logger.Info("file given with zero content length, assuming missing file and will not create", "path", file)
		}
		wg.Done()
		logger.Info("file downloaded", "path", file)
		w.WriteHeader(http.StatusOK)
	}
}

func Receive(ctx context.Context, files []string) error {
	logger := logf.FromContext(ctx)

	port := os.Getenv(v1beta1.EnvVarExtractorPort)
	var (
		mux   = http.NewServeMux()
		locks = make(filelocks)
		wg    = &sync.WaitGroup{}
	)

	v, err := newJWTValidator()
	if err != nil {
		logger.Error(err, "unable to instaniate JWT validator")
		os.Exit(1)
	}

	for _, file := range files {
		locks[file] = &sync.Mutex{}
		wg.Add(1)
	}
	mux.HandleFunc("/upload/", uploadHandler(v, wg, locks))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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
		}
	}()
	logger.Info("awaiting file downloads", "count", len(files))
	wg.Wait()
	logger.Info("all files downloaded, shutting down server")

	if err = srv.Shutdown(ctx); err != nil {
		logger.Error(err, "Error shutting down server from completion")
	}
	return nil
}

type jwtValidator struct {
	mu        sync.RWMutex
	keySet    *jose.JSONWebKeySet
	fetchedAt time.Time
	ttl       time.Duration
	client    *http.Client
}

func (v *jwtValidator) getKeyset(ctx context.Context) (*jose.JSONWebKeySet, error) {
	// fast path — valid cache
	v.mu.RLock()
	if v.keySet != nil && time.Since(v.fetchedAt) < v.ttl {
		ks := v.keySet
		v.mu.RUnlock()
		return ks, nil
	}
	v.mu.RUnlock()
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.keySet != nil && time.Since(v.fetchedAt) < v.ttl {
		return v.keySet, nil
	}

	keySet, err := fetchJWKS(ctx, v.client)
	if err != nil {
		if v.keySet != nil {
			return v.keySet, nil
		}
		return nil, err
	}

	v.keySet = keySet
	v.fetchedAt = time.Now()
	return v.keySet, nil

}

func (v *jwtValidator) validateToken(ctx context.Context, rawToken string) (*k8sClaims, error) {
	keyset, err := v.getKeyset(ctx)
	if err != nil {
		return nil, err
	}
	tok, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims := &k8sClaims{}
	if err := tok.Claims(keyset, claims); err != nil {
		return nil, fmt.Errorf("extracting claims: %w", err)
	}

	return claims, nil
}

type k8sClaims struct {
	jwt.Claims `json:",inline"`
	Kubernetes struct {
		Namespace string `json:"namespace"`
		Pod       *struct {
			Name string `json:"name"`
			UID  string `json:"uid"`
		} `json:"pod,omitempty"`
		ServiceAccount struct {
			Name string `json:"name"`
			UID  string `json:"uid"`
		} `json:"serviceaccount"`
	} `json:"kubernetes.io"`
}

const (
	jwksURL   = "https://kubernetes.default.svc/openid/v1/jwks"
	caPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func newJWTValidator() (*jwtValidator, error) {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("reading cluster CA: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse cluster CA cert")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}

	return &jwtValidator{
		client: httpClient,
	}, nil
}

func fetchJWKS(ctx context.Context, client *http.Client) (*jose.JSONWebKeySet, error) {
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("reading SA token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	defer utils.CloseAndLog(ctx, resp.Body, "closing JWKs client")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading JWKS response: %w", err)
	}

	var keySet jose.JSONWebKeySet
	if err := json.Unmarshal(body, &keySet); err != nil {
		return nil, fmt.Errorf("parsing JWKS: %w", err)
	}

	return &keySet, nil
}
