// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package validators

import (
	"errors"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
)

var ErrVolumeCollision = errors.New("volume name collision detected")

func DetectVolumeCollision(volumes []v1.Volume, mergeDuplicates bool) ([]v1.Volume, error) {
	deduplicatedVolumes := make(map[string]v1.Volume, len(volumes))
	for _, vol := range volumes {
		if existingVol, exists := deduplicatedVolumes[vol.Name]; exists {
			if mergeDuplicates && reflect.DeepEqual(existingVol, vol) {
				continue
			}
			return nil, fmt.Errorf("%w: duplicate volumes with name %s", ErrVolumeCollision, vol.Name)
		} else {
			deduplicatedVolumes[vol.Name] = vol
		}
	}

	// Convert map back to slice
	result := make([]v1.Volume, 0, len(deduplicatedVolumes))
	for _, vol := range deduplicatedVolumes {
		result = append(result, vol)
	}
	return result, nil
}
