// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package identities provides the [Identity] type which
// represents the identity of a user in the cluster.
// It also provides the [AuthenticateToken] and [AuthenticateClient]
// functions which are used to authenticate a token or client certificate
// using the Kubernetes API server. Additionally, it can determine the
// authorization of the user using the [Authorize] function.
package identities

import (
	authv1 "k8s.io/api/authentication/v1"
)

// Identity represents the identity of a user in the cluster
// and contains the user information.
type Identity struct {
	User      authv1.UserInfo
	Audiences []string
}
