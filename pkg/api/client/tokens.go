// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package client

import (
	"net/http"
	"os"
	"sync"
	"time"
)

type tokenRetriever interface {
	GetToken() (string, time.Time, error)
}

type staticToken struct {
	token string
}

func (s staticToken) GetToken() (string, time.Time, error) {
	return s.token, time.Time{}, nil
}

type tokenFileRetriever struct {
	file    string
	refresh time.Duration
}

func (t tokenFileRetriever) GetToken() (string, time.Time, error) {
	data, err := os.ReadFile(t.file)
	if err != nil {
		return "", time.Time{}, err
	}

	return string(data), time.Now().Add(t.refresh), nil
}

type tokenFileTransport struct {
	token     string
	nextCheck time.Time
	base      http.RoundTripper
	mu        sync.Mutex
	retriever tokenRetriever
}

func (t *tokenFileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if time.Now().After(t.nextCheck) {
		token, nextCheck, err := t.retriever.GetToken()
		if err != nil {
			return nil, err
		}
		t.token = token
		t.nextCheck = nextCheck
	}

	req.Header.Set("Authorization", "Bearer "+t.token)
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	return t.base.RoundTrip(req)
}
