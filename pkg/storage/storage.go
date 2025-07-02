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
	"encoding"

	"github.com/crashappsec/ocular/pkg/cluster"
	"gopkg.in/yaml.v3"
)

// Object is an interface that represents a generic object that
// defines user data. It is the value that will be associated
// with a key and stored by a [Backend]. The object will be marshalled and unmarshalled
// using functions from the [yaml] library, unless it implements the
// [encoding.TextMarshaler] or [encoding.TextUnmarshaler] interfaces in which case
// those will be used instead.
type Object interface {
	// Validate validates the object. It will be given a [cluster.Context]
	// for the current users context and can reach out to the cluster API
	// to validate the object. If the object is not valid, it should return
	// a non-nil error.
	Validate(ctx context.Context, clusterCtx cluster.Context) error
}

// marshallObject marshals the given object into a byte slice.
// if the object implements the [encoding.TextMarshaler] interface,
// it will use that to marshal the object.
func marshallObject(obj any) ([]byte, error) {
	if obj == nil {
		return nil, nil
	}
	switch o := obj.(type) {
	case encoding.TextMarshaler:
		return o.MarshalText()
	default:
		return yaml.Marshal(obj)
	}
}

// unmarshallObject unmarshals the given byte slice into the specified object.
// if the object implements the [encoding.TextUnmarshaler] interface,
// it will use that to unmarshal the object.
func unmarshallObject(data []byte, obj any) error {
	if obj == nil {
		return nil
	}
	switch o := obj.(type) {
	case encoding.TextUnmarshaler:
		return o.UnmarshalText(data)
	default:
		return yaml.Unmarshal(data, obj)
	}
}
