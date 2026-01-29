// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package containers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type Option = func(*corev1.Container)

func WithAdditionalEnvVars(envs ...corev1.EnvVar) Option {
	return func(c *corev1.Container) {
		c.Env = append(c.Env, envs...)
	}
}

func WithAdditionalArgs(args ...string) Option {
	return func(c *corev1.Container) {
		c.Args = append(c.Args, args...)
	}
}

func WithAdditionalVolumeMounts(mounts ...corev1.VolumeMount) Option {
	return func(c *corev1.Container) {
		c.VolumeMounts = append(c.VolumeMounts, mounts...)
	}
}

func WithWorkingDir(dir string) Option {
	return func(c *corev1.Container) {
		c.WorkingDir = dir
	}
}

func ApplyOptions(
	containers []corev1.Container,
	options ...Option,
) []corev1.Container {
	for i := range containers {
		for _, option := range options {
			option(&containers[i])
		}
	}
	return containers
}

func WithPodSecurityStandardRestricted() Option {
	return func(c *corev1.Container) {
		if c.SecurityContext == nil {
			c.SecurityContext = &corev1.SecurityContext{}
		}
		c.SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
		c.SecurityContext.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
		c.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}
}
