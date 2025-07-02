// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package identities

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/crashappsec/ocular/pkg/cluster"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
)

// AuthenticateToken authenticates a token using the Kubernetes API server.
// It returns the identity of the user and their audiences. It will
func AuthenticateToken(
	ctx context.Context,
	reviewInterface v1.TokenReviewInterface,
	token string,
) (Identity, error) {
	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
			Audiences: []string{
				string(TokenAudienceCrawler),
				// additionally include the API server audience
				string(TokenAudienceKubernetesDefault),
				string(TokenAudienceKubernetesDefaultLocal),
			},
		},
	}

	review, err := reviewInterface.Create(ctx, tokenReview, metav1.CreateOptions{})
	if err != nil {
		return Identity{}, err
	}

	if !review.Status.Authenticated {
		return Identity{}, fmt.Errorf("token is not authenticated: %s", review.Status.Error)
	}

	return Identity{
		User:      review.Status.User,
		Audiences: review.Spec.Audiences,
	}, nil
}

// AuthenticateClient authenticates a client certificate using the Kubernetes API server.
// It returns the identity of the user and their audiences. It will also check if the
// certificate is valid and not expired.
func AuthenticateClient(
	ctx context.Context,
	clusterCtx cluster.Context,
	cert *x509.Certificate,
) (Identity, error) {
	// TODO(bryce) this will need to be validated against clusterContext public key

	userName := cert.Subject.CommonName
	groups := cert.Subject.Organization
	return Identity{
		User: authv1.UserInfo{
			Username: userName,
			Groups:   groups,
		},
	}, nil
}
