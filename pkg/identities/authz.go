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
	"slices"
	"strings"

	"github.com/crashappsec/ocular/pkg/cluster"
	"go.uber.org/zap"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Authorizer is a function that checks if a user is authorized to access a resource.
// It is used in middleware to check if a user is authorized to access a resource.
type Authorizer = func(ctx context.Context, clusterCtx cluster.Context, identity Identity) (bool, error)

// AudienceAuthorizer checks if the user has the specified audience.
// This should be used by generating projected service account tokens
// via the method CreateTokenVolume.
func AudienceAuthorizer(audience Audience) Authorizer {
	return func(ctx context.Context, clusterCtx cluster.Context, identity Identity) (bool, error) {
		l := zap.L().With(
			zap.String("authorizer", "audience"),
			zap.String("audience", string(audience)),
			zap.Any("identity", identity),
		)
		l.Debug("authorizing user audience")

		if !strings.HasPrefix(
			identity.User.Username,
			"system:serviceaccount:"+clusterCtx.Namespace+":",
		) {
			l.Debug("user does not originate from the namespace of the cluster context")
			return false, nil
		}

		if slices.Contains(identity.Audiences, string(audience)) {
			l.Debug("user is authorized to access resource via audience")
			return true, nil
		}

		l.Debug("user is not authorized to access resource via lack of audience")
		return false, nil
	}
}

// PermissionSet represents a set of permissions to check against
type PermissionSet struct {
	Group    string
	Resource string
	Verb     string
}

// PermissionAuthorizer checks if the user has the specified permissions
// by creating a SubjectAccessReview for each permission.
func PermissionAuthorizer(permissions ...PermissionSet) Authorizer {
	return func(
		ctx context.Context,
		clusterCtx cluster.Context,
		identity Identity,
	) (bool, error) {
		l := zap.L().With(
			zap.String("authorizer", "permissions"),
			zap.Int("permissions_amount", len(permissions)),
		)
		l.Debug("authorizing user")

		for _, permission := range permissions {
			ll := l.With(zap.Any("permission", permission))
			sar := &authzv1.SubjectAccessReview{
				Spec: authzv1.SubjectAccessReviewSpec{
					User: identity.User.Username,
					ResourceAttributes: &authzv1.ResourceAttributes{
						Group:     permission.Group,
						Namespace: clusterCtx.Namespace,
						Verb:      permission.Verb,
						Resource:  permission.Resource,
					},
				},
			}

			resp, err := clusterCtx.CS.AuthorizationV1().
				SubjectAccessReviews().
				Create(ctx, sar, metav1.CreateOptions{})
			if err != nil {
				return false, err
			}
			ll.Debug("received server response", zap.Bool("allowed", resp.Status.Allowed))
			if !resp.Status.Allowed {
				ll.Debug("user is not authorized to access resource")
				return false, nil
			}
		}

		l.Debug("user is authorized to access resources")
		return true, nil
	}
}
