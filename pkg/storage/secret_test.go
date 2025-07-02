// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package storage

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/crashappsec/ocular/internal/unittest/mocks"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

func TestSecret_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	cm := mocks.NewMockSecretInterface(ctrl)

	tests := []struct {
		name          string
		backend       Backend[dummyObject]
		registerMocks func(m *mocks.MockSecretInterface)
		key           string
		expectedValue string
		wantErr       bool
	}{
		{
			name: "get from Secret",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret1",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret1"), gomock.Eq(metav1.GetOptions{})).
					Return(&v1.Secret{Data: map[string][]byte{
						"test1": []byte("!!test"),
					}}, nil).Times(1)
			},
			key:           "test1",
			expectedValue: "test",
		},
		{
			name: "failed to marshall from Secret",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret3",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret3"), gomock.Eq(metav1.GetOptions{})).
					Return(&v1.Secret{Data: map[string][]byte{
						"test3": []byte("not empty object"),
					}}, nil).Times(1)
			},
			wantErr: true,
			key:     "test3",
		},
		{
			name: "Secret retrieval error",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret4",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret4"), gomock.Eq(metav1.GetOptions{})).
					Return(nil, fmt.Errorf("unknown error")).Times(1)
			},
			wantErr: true,
			key:     "test4",
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.registerMocks != nil {
				test.registerMocks(cm)
			}

			val, err := test.backend.Get(ctx, test.key)
			if test.wantErr && err == nil {
				t.Errorf("Secret.Get() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && err != nil {
				t.Errorf("Secret.Get() error = %v, want nil", err)
			}

			if !test.wantErr && test.expectedValue != val.value {
				t.Errorf("Secret.Get() = %v, want %v", val, test.expectedValue)
			}
		})
	}
}

func TestSecret_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	cm := mocks.NewMockSecretInterface(ctrl)

	tests := []struct {
		name           string
		backend        Backend[dummyObject]
		registerMocks  func(m *mocks.MockSecretInterface)
		wantErr        bool
		expectedValues map[string]string
	}{
		{
			name: "get from Secret",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret1",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret1"), gomock.Eq(metav1.GetOptions{})).
					Return(&v1.Secret{Data: map[string][]byte{
						"test2": []byte("!!2"),
						"test3": []byte("!!3"),
					}}, nil).Times(1)
			},
			expectedValues: map[string]string{
				"test2": "2",
				"test3": "3",
			},
		},
		{
			name: "failed to marshall from Secret",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret3",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret3"), gomock.Eq(metav1.GetOptions{})).
					Return(&v1.Secret{Data: map[string][]byte{
						"test7": []byte("!!7"),
						"test8": []byte("not empty object"),
						"test9": []byte("!!8"),
					}}, nil).Times(1)
			},
			wantErr: true,
		},
		{
			name: "Secret retrieval error",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret4",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().
					Get(gomock.Any(), gomock.Eq("Secret4"), gomock.Eq(metav1.GetOptions{})).
					Return(nil, fmt.Errorf("unknown error")).Times(1)
			},
			wantErr: true,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.registerMocks != nil {
				test.registerMocks(cm)
			}

			items, err := test.backend.List(ctx)
			if test.wantErr && err == nil {
				t.Errorf("Secret.Get() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && err != nil {
				t.Errorf("Secret.Get() error = %v, want nil", err)
			}

			if !test.wantErr {
				for key, expectedVal := range test.expectedValues {
					val, ok := items[key]
					if !ok {
						t.Errorf("Expected item %s not found in result", key)
					}
					if expectedVal != val.value {
						t.Errorf(
							"Expected value for %s to be %s, got %s",
							key,
							expectedVal,
							val.value,
						)
					}
				}
			}
		})
	}
}

func TestSecret_Set(t *testing.T) {
	ctrl := gomock.NewController(t)
	cm := mocks.NewMockSecretInterface(ctrl)

	tests := []struct {
		name          string
		backend       Backend[dummyObject]
		registerMocks func(m *mocks.MockSecretInterface)
		key           string
		value         dummyObject
		wantErr       bool
	}{
		{
			name: "set to Secret",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret1",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				b64Secret := base64.StdEncoding.EncodeToString([]byte("!!secret"))
				m.EXPECT().Patch(
					gomock.Any(),
					gomock.Eq("Secret1"),
					gomock.Eq(k8sTypes.MergePatchType),
					gomock.Eq([]byte(`{"data":{"test1":"`+b64Secret+`"}}`)),
					gomock.Eq(metav1.PatchOptions{}),
				).Return(&v1.Secret{}, nil).Times(1)
			},
			key:     "test1",
			value:   dummyObject{value: "secret"},
			wantErr: false,
		},
		{
			name: "set to Secret error",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret1",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				b64Secret := base64.StdEncoding.EncodeToString([]byte("!!secret"))
				m.EXPECT().Patch(
					gomock.Any(),
					gomock.Eq("Secret1"),
					gomock.Eq(k8sTypes.MergePatchType),
					gomock.Eq([]byte(`{"data":{"test1":"`+b64Secret+`"}}`)),
					gomock.Eq(metav1.PatchOptions{}),
				).Return(nil, fmt.Errorf("unknown error")).Times(1)
			},
			key:     "test1",
			value:   dummyObject{value: "secret"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.registerMocks != nil {
				test.registerMocks(cm)
			}

			err := test.backend.Set(context.Background(), test.key, test.value)
			if test.wantErr && err == nil {
				t.Errorf("Secret.Set() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && err != nil {
				t.Errorf("Secret.Set() error = %v, want nil", err)
			}
		})
	}
}

func TestSecret_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	cm := mocks.NewMockSecretInterface(ctrl)

	tests := []struct {
		name          string
		backend       Backend[dummyObject]
		registerMocks func(m *mocks.MockSecretInterface)
		key           string
		wantErr       bool
	}{
		{
			name: "delete Secret value",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret1",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().Patch(
					gomock.Any(),
					gomock.Eq("Secret1"),
					gomock.Eq(k8sTypes.MergePatchType),
					gomock.Eq([]byte(`{"data":{"test1":null}}`)),
					gomock.Eq(metav1.PatchOptions{}),
				).Return(&v1.Secret{}, nil).Times(1)
			},
			key: "test1",
		},
		{
			name: "delete Secret error",
			backend: &SecretStore[dummyObject]{
				s:    cm,
				name: "Secret3",
			},
			registerMocks: func(m *mocks.MockSecretInterface) {
				m.EXPECT().Patch(
					gomock.Any(),
					gomock.Eq("Secret3"),
					gomock.Eq(k8sTypes.MergePatchType),
					gomock.Eq([]byte(`{"data":{"test3":null}}`)),
					gomock.Eq(metav1.PatchOptions{}),
				).Return(nil, fmt.Errorf("unknown err")).Times(1)
			},
			key:     "test3",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.registerMocks != nil {
				test.registerMocks(cm)
			}

			err := test.backend.Delete(context.Background(), test.key)
			if test.wantErr && err == nil {
				t.Errorf("Secret.Delete() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && err != nil {
				t.Errorf("Secret.Delete() error = %v, want nil", err)
			}
		})
	}
}
