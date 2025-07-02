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
	"math/rand"
	"testing"

	"github.com/crashappsec/ocular/internal/unittest"
	"github.com/crashappsec/ocular/internal/unittest/mocks"
	storageMock "github.com/crashappsec/ocular/internal/unittest/mocks/pkg/storage"
	"github.com/crashappsec/ocular/pkg/resources"
	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	batchV1 "k8s.io/api/batch/v1"
)

func TestRun(t *testing.T) {
	ctrl := gomock.NewController(t)

	uuid.EnableRandPool()
	t.Cleanup(func() {
		uuid.DisableRandPool()
	})
	uuid.SetRand(rand.New(rand.NewSource(0))) // #nosec G404
	uuid1 := uuid.MustParse("0194fdc2-fa2f-4cc0-81d3-ff12045b73c8")

	ctx := context.Background()
	jobInterface := mocks.NewMockJobInterface(ctrl)
	serviceInterface := mocks.NewMockServiceInterface(ctrl)
	uploaderStorageBackend := storageMock.NewMockBackend[resources.Uploader](ctrl)
	svcNamespace := "namespace-" + unittest.GenerateRandStr(unittest.CharSetAlpha, 10)
	profileName := "profile-" + unittest.GenerateRandStr(unittest.CharSetAlpha, 10)

	tests := []struct {
		name          string
		registerMocks func(
			jobInterface *mocks.MockJobInterface,
			serviceInterface *mocks.MockServiceInterface,
			uploaderStorageBackend *storageMock.MockBackend[resources.Uploader],
		)
		target           schemas.Target
		profile          resources.Profile
		downloader       resources.Downloader
		expectedErr      error
		expectedPipeline Pipeline
	}{
		{
			name: "valid pipeline, 1 scanner, no uploaders",
			registerMocks: func(
				jobInterface *mocks.MockJobInterface,
				_ *mocks.MockServiceInterface,
				_ *storageMock.MockBackend[resources.Uploader],
			) {
				// TODO(bryce): eventually match the job spec
				// expectedJob := &batchV1.Job{
				//	 ObjectMeta: metav1.ObjectMeta{
				//	 	Name: "pipeline-" + uuid1.String(),
				//	 	Labels: map[string]string{
				//	 		runtime.LabelPipelineID: uuid1.String(),
				//	 	},
				//	 },
				// }
				jobInterface.EXPECT().
					Create(gomock.Any(), gomock.AssignableToTypeOf(&batchV1.Job{}), gomock.Any()).
					Return(nil, nil).
					Times(1)
			},
			expectedPipeline: Pipeline{
				ID: uuid1,
				Target: schemas.Target{
					Identifier: "test1",
					Downloader: "test-downloader",
				},
				Profile:      profileName,
				ScanStatus:   schemas.RunStatusPending,
				UploadStatus: schemas.RunStatusNotRan,
			},
			target: schemas.Target{
				Identifier: "test1",
				Downloader: "test-downloader",
			},
			profile: resources.Profile{
				Scanners: []schemas.Scanner{
					{
						Image: "test-image",
						Command: []string{
							"/bin/my-scanner",
						},
						Args: []string{
							"--arg1",
							"--arg2",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.registerMocks(
				jobInterface,
				serviceInterface,
				uploaderStorageBackend,
			)
			pline, err := Run(
				ctx,
				jobInterface,
				serviceInterface,
				svcNamespace,
				test.target,
				test.downloader,
				profileName,
				test.profile,
				uploaderStorageBackend,
			)

			if test.expectedErr != nil {
				assert.ErrorIs(t, err, test.expectedErr)
			} else {
				assert.Equal(t, test.expectedPipeline, pline)
			}
		})
	}
}
