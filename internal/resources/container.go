// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type ContainerRunRequest struct {
	Spec corev1.Container `json:"spec,omitempty" description:"Container specification"`
	// Volumes is a list of volumes to add to the pod.
	Volumes []corev1.Volume `json:"volumes,omitempty" description:"A list of volumes to add to the pod."`
}

type ContainerOption = func(*corev1.Container)

func ContainerWithAdditionalEnvVars(envs ...corev1.EnvVar) ContainerOption {
	return func(c *corev1.Container) {
		c.Env = append(c.Env, envs...)
	}
}

func ContainerWithAdditionalArgs(args ...string) ContainerOption {
	return func(c *corev1.Container) {
		c.Args = append(c.Args, args...)
	}
}

func ContainerWithAdditionalVolumeMounts(mounts ...corev1.VolumeMount) ContainerOption {
	return func(c *corev1.Container) {
		c.VolumeMounts = append(c.VolumeMounts, mounts...)
	}
}

func ContainerWithWorkingDir(dir string) ContainerOption {
	return func(c *corev1.Container) {
		c.WorkingDir = dir
	}
}

func ApplyOptionsToContainers(
	containers []corev1.Container,
	options ...ContainerOption,
) []corev1.Container {
	for i := range containers {
		for _, option := range options {
			option(&containers[i])
		}
	}
	return containers
}

func ContainerWithPodSecurityStandardRestricted() ContainerOption {
	return func(c *corev1.Container) {
		if c.SecurityContext == nil {
			c.SecurityContext = &corev1.SecurityContext{}
		}
		c.SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
		// c.SecurityContext.RunAsNonRoot = ptr.To(true)
		// c.SecurityContext.RunAsUser = ptr.To[int64](65538)
		c.SecurityContext.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
		c.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}
}
