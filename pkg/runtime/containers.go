// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package runtime provides functions to manage the
// runtime environment of the application. It will provide
// abstractions around the Kubernetes runtime environment,
// creating an executing containers, and managing the lifecycle
// of the application.
package runtime

import (
	"strings"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/pkg/schemas"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

type ContainerOpt = func(v1.Container) v1.Container

func ContainerWorkingDirOpt(workingDir string) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.WorkingDir = workingDir
		return ctr
	}
}

func ContainerEnvOpt(envVars ...v1.EnvVar) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.Env = append(ctr.Env, envVars...)
		return ctr
	}
}

func ContainerVolumesOpt(volumes ...v1.VolumeMount) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.VolumeMounts = append(ctr.VolumeMounts, volumes...)
		return ctr
	}
}

func ContainerArgsOpt(args ...string) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.Args = append(ctr.Args, args...)
		return ctr
	}
}

func ContainerRestartPolicyOpt(restartPolicy v1.ContainerRestartPolicy) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.RestartPolicy = ptr.To[v1.ContainerRestartPolicy](restartPolicy)
		return ctr
	}
}

func ContainerPortOpt(ports ...v1.ContainerPort) ContainerOpt {
	return func(ctr v1.Container) v1.Container {
		ctr.Ports = append(ctr.Ports, ports...)
		return ctr
	}
}

// CreateContainer creates a new container with the given name and configuration.
// It will create the container based on the provided [types.UserContainer] and apply the
// [ContainerOpt] options to it. For any secrets that are mounted as files, it will
// create a volume mount for the secret and configure it to come from the volume with the
// name [secretVolumeName].
func CreateContainer(
	name string,
	container schemas.UserContainer,
	secretVolumeName string,
	opts ...ContainerOpt,
) v1.Container {
	ctr := v1.Container{
		Name:            name,
		Image:           container.Image,
		ImagePullPolicy: v1.PullPolicy(container.ImagePullPolicy),
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{},
			Limits:   v1.ResourceList{},
		},
	}

	if config.State.Runtime.Requests.CPU != "" {
		cpuReq, err := resource.ParseQuantity(config.State.Runtime.Requests.CPU)
		if err != nil {
			zap.L().Error("failed to parse CPU request", zap.Error(err))
		} else {
			ctr.Resources.Requests[v1.ResourceCPU] = cpuReq
		}
	}

	if config.State.Runtime.Requests.Memory != "" {
		memoryReq, err := resource.ParseQuantity(config.State.Runtime.Requests.Memory)
		if err != nil {
			zap.L().Error("failed to parse CPU request", zap.Error(err))
		} else {
			ctr.Resources.Requests[v1.ResourceMemory] = memoryReq
		}
	}

	if len(container.Command) > 0 {
		ctr.Command = container.Command
	}

	if len(container.Args) > 0 {
		ctr.Args = container.Args
	}

	if len(container.Env) > 0 {
		var customEnv []v1.EnvVar
		for _, env := range container.Env {
			if strings.HasPrefix(env.Name, schemas.EnvVarPrefix) {
				// not sure what to do in this case, adding this it is probably best
				env.Name = schemas.CustomEnvVarPrefix + env.Name
			}
			customEnv = append(customEnv, v1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
		ctr.Env = append(ctr.Env, customEnv...)
	}

	if len(container.Secrets) > 0 {
		for _, secretRef := range container.Secrets {
			switch secretRef.MountType {
			case schemas.SecretMountTypeFile:
				ctr.VolumeMounts = append(ctr.VolumeMounts, v1.VolumeMount{
					Name:      secretVolumeName,
					MountPath: secretRef.MountTarget,
					SubPath:   secretRef.Name,
				})
			case schemas.SecretMountTypeEnvVar:
				ctr.Env = append(ctr.Env, v1.EnvVar{
					Name: secretRef.MountTarget,
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{
								Name: config.State.Secrets.SecretName,
							},
							Key:      secretRef.Name,
							Optional: ptr.To(!secretRef.Required),
						},
					},
				})
			default:
				zap.L().Warn("secret mount type not supported")
				continue
			}
		}
	}

	for _, opt := range opts {
		ctr = opt(ctr)
	}

	return ctr
}
