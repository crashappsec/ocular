// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package runtime

import (
	"testing"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/internal/unittest"
	"github.com/crashappsec/ocular/pkg/schemas"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestCreateContainer(t *testing.T) {
	baseContainer := schemas.UserContainer{
		Image:           "test-image:" + unittest.GenerateRandStr(unittest.CharSetAlpha, 5),
		ImagePullPolicy: "IfNotPresent",
		Command:         []string{"/bin/shell"},
		Args: []string{
			"program",
			"--arg0", unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 10),
			"--arg1", unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 10),
		},
		Env: []schemas.EnvVar{
			{
				Name:  "ENV_VAR_0",
				Value: unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 10),
			},
			{
				Name:  "ENV_VAR_1",
				Value: unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 10),
			},
		},
		Secrets: []schemas.SecretRef{
			{
				Name:        "test-env-secret",
				MountTarget: "SECRET_ENV",
				MountType:   schemas.SecretMountTypeEnvVar,
			},
			{
				Name:        "test-file-secret",
				MountTarget: "/secret/file",
				MountType:   schemas.SecretMountTypeFile,
				Required:    true,
			},
		},
	}

	secretVolumeName := "secret-vol-" + unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 5)
	secretStoreName := "secret-store-" + unittest.GenerateRandStr(unittest.CharSetAlphaNumeric, 5)

	old := config.State.Secrets.SecretName
	config.State.Secrets.SecretName = secretStoreName
	t.Cleanup(func() {
		config.State.Secrets.SecretName = old
	})
	tests := []struct {
		name             string
		containerName    string
		container        schemas.UserContainer
		secretVolumeName string
		opts             []ContainerOpt
		expected         v1.Container
	}{
		{
			name:             "test with no options",
			containerName:    "test-container",
			container:        baseContainer,
			secretVolumeName: secretVolumeName,
			opts:             nil,
			expected: v1.Container{
				Name:            "test-container",
				Image:           baseContainer.Image,
				ImagePullPolicy: v1.PullPolicy(baseContainer.ImagePullPolicy),
				Args:            baseContainer.Args,
				Command:         baseContainer.Command,
				Env: []v1.EnvVar{
					{
						Name:  "ENV_VAR_0",
						Value: baseContainer.Env[0].Value,
					},
					{
						Name:  "ENV_VAR_1",
						Value: baseContainer.Env[1].Value,
					},
					{
						Name: "SECRET_ENV",
						ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: secretStoreName,
							},
							Key:      "test-env-secret",
							Optional: ptr.To(true),
						}},
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      secretVolumeName,
						MountPath: "/secret/file",
						SubPath:   "test-file-secret",
					},
				},
			},
		},
		{
			name:             "test with options",
			containerName:    "test-container",
			container:        baseContainer,
			secretVolumeName: secretVolumeName,
			opts: []ContainerOpt{
				ContainerEnvOpt(v1.EnvVar{Name: "ENV_VAR_2", Value: "added-later"}),
				ContainerArgsOpt("--additional arg"),
				ContainerPortOpt(v1.ContainerPort{ContainerPort: 8080}),
				ContainerWorkingDirOpt("/new/dir"),
			},
			expected: v1.Container{
				Name:            "test-container",
				Image:           baseContainer.Image,
				ImagePullPolicy: v1.PullPolicy(baseContainer.ImagePullPolicy),
				Args:            append(baseContainer.Args, "--additional arg"),
				Command:         baseContainer.Command,
				WorkingDir:      "/new/dir",
				Env: []v1.EnvVar{
					{
						Name:  "ENV_VAR_0",
						Value: baseContainer.Env[0].Value,
					},
					{
						Name:  "ENV_VAR_1",
						Value: baseContainer.Env[1].Value,
					},
					{
						Name: "SECRET_ENV",
						ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: secretStoreName,
							},
							Key:      "test-env-secret",
							Optional: ptr.To(true),
						}},
					},
					{
						Name:  "ENV_VAR_2",
						Value: "added-later",
					},
				},
				Ports: []v1.ContainerPort{
					{
						ContainerPort: 8080,
					},
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      secretVolumeName,
						MountPath: "/secret/file",
						SubPath:   "test-file-secret",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := CreateContainer(
				"test-container",
				tt.container,
				tt.secretVolumeName,
				tt.opts...)
			unittest.AssertEqualContainers(t, tt.expected, ctr)
		})
	}
}
