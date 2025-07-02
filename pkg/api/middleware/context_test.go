// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crashappsec/ocular/internal/unittest"
	clusterMock "github.com/crashappsec/ocular/internal/unittest/mocks/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/cluster"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func TestContextAssigner(t *testing.T) {
	ctrl := gomock.NewController(t)
	cm := clusterMock.NewMockContextManager(ctrl)

	randString0 := unittest.GenerateRandStr(unittest.CharSetAll, 10)
	randString1 := unittest.GenerateRandStr(unittest.CharSetAll, 10)
	tests := []struct {
		name            string
		registerMocks   func(m *clusterMock.MockContextManager)
		headers         map[string]string
		expectedContext cluster.Context
		expectedStatus  int
	}{
		{
			name: "no header, default context enabled",
			registerMocks: func(m *clusterMock.MockContextManager) {
				m.EXPECT().DefaultContext().Return(
					cluster.Context{Name: "default", Namespace: randString0}, true,
				).Times(1)
			},
			expectedContext: cluster.Context{Name: "default", Namespace: randString0},
		},
		{
			name: "no header, default context not enabled",
			registerMocks: func(m *clusterMock.MockContextManager) {
				m.EXPECT().DefaultContext().Return(
					cluster.Context{}, false,
				).Times(1)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "header set, context exists",
			headers: map[string]string{
				schemas.ClusterContextHeader: "test123",
			},
			registerMocks: func(m *clusterMock.MockContextManager) {
				m.EXPECT().GetContext("test123").Return(
					cluster.Context{
						Name:      "test123",
						Namespace: randString1,
					}, true,
				).Times(1)
			},
			expectedContext: cluster.Context{
				Name:      "test123",
				Namespace: randString1,
			},
		},
		{
			name: "header set, context does not exist",
			headers: map[string]string{
				schemas.ClusterContextHeader: "non-existent",
			},
			registerMocks: func(m *clusterMock.MockContextManager) {
				m.EXPECT().GetContext("non-existent").Return(
					cluster.Context{}, false,
				).Times(1)
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	r := gin.New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c := gin.CreateTestContextOnly(w, r)
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			c.Request = req
			// Register mocks
			if tt.registerMocks != nil {
				tt.registerMocks(cm)
			}
			ContextAssigner(cm)(c)

			expectedStatus := http.StatusOK
			if tt.expectedStatus != 0 {
				expectedStatus = tt.expectedStatus
			}

			if w.Code != expectedStatus {
				t.Errorf("expected status code %d, got %d", expectedStatus, w.Code)
			}

			// Check the context
			if tt.expectedContext != (cluster.Context{}) {
				ctx, exists := GetClusterContext(c)
				if !exists {
					t.Errorf("expected context to be set")
				}
				if ctx.Name != tt.expectedContext.Name ||
					ctx.Namespace != tt.expectedContext.Namespace {
					t.Errorf("expected context %v, got %v", tt.expectedContext, ctx)
				}
			} else {
				_, exists := GetClusterContext(c)
				if exists {
					t.Errorf("expected context to not be set")
				}
			}
		})
	}
}
