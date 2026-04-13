// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	"github.com/crashappsec/ocular/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func parseNewRequiredParameters(old []v1beta1.ParameterDefinition, new []v1beta1.ParameterDefinition) []v1beta1.ParameterDefinition {
	var introduced []v1beta1.ParameterDefinition

	oldParamSet := make(map[string]struct{})
	for _, oldParam := range old {
		oldParamSet[oldParam.Name] = struct{}{}
	}

	for _, newParam := range new {
		if _, found := oldParamSet[newParam.Name]; !found && newParam.Default == nil {
			introduced = append(introduced, newParam)
		}
	}
	return introduced
}

func refMatches(ref v1beta1.ParameterizedObjectReference, obj client.Object, objKind string) bool {
	return ref.Name == obj.GetName() &&
		ref.Namespace == obj.GetNamespace() &&
		ref.Kind == objKind
}
