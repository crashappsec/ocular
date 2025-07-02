// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package identities

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Audience represents the audience for which the token is intended.
type Audience string

const (
	// TokenAudienceCrawler is the audience for the crawler service account.
	TokenAudienceCrawler Audience = "crashoverride.run/crawler"
	// TokenAudienceKubernetesDefaultLocal is the audience for the Kubernetes API server using the
	// 'svc.cluster.local' domain.
	TokenAudienceKubernetesDefaultLocal Audience = "https://kubernetes.default.svc.cluster.local" // #nosec G101
	// TokenAudienceKubernetesDefault is the audience for the Kubernetes API server using the
	// 'svc' domain.
	TokenAudienceKubernetesDefault Audience = "https://kubernetes.default.svc" // #nosec G101

	// TokenMountPath is the mount path for the token volume.
	TokenMountPath = "/var/run/secrets" // #nosec G101
	// TokenFileName is the name of the token file.
	TokenFileName = "ocular-api-token" // #nosec G101
	// TokenFilePath is the full path to the token file.
	TokenFilePath = TokenMountPath + "/" + TokenFileName

	// TokenValiditySeconds is the number of seconds the token will be valid for.
	TokenValiditySeconds = int(time.Minute * 60 / time.Second)
)

// CreateTokenVolume creates a projected service account token volume
// for the specified audience. The token will be mounted as the file [TokenFilePath] and
// will be valid for [TokenValiditySeconds] seconds. The function returns the
// tokenpath, the volume, and the volume mount for the token.
func CreateTokenVolume(aud Audience) (string, v1.Volume, v1.VolumeMount) {
	vol := v1.Volume{
		Name: "ocular-token-volume",
		VolumeSource: v1.VolumeSource{
			Projected: &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{
					{
						ServiceAccountToken: &v1.ServiceAccountTokenProjection{
							Audience:          string(aud),
							Path:              TokenFileName,
							ExpirationSeconds: ptr.To(int64(TokenValiditySeconds)),
						},
					},
				},
			},
		},
	}

	volMount := v1.VolumeMount{
		Name:      vol.Name,
		MountPath: TokenMountPath,
	}
	return TokenFilePath, vol, volMount
}
