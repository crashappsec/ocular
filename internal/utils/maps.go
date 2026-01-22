// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utils

import "maps"

// MergeMaps merges two maps, with the values from the second map
// overriding the values from the first map.
func MergeMaps[K comparable, V any](base, override map[K]V) map[K]V {
	merged := make(map[K]V, len(base)+len(override))

	maps.Copy(merged, base)

	maps.Copy(merged, override)

	return merged
}
