// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package unittest

import "math/rand"

type CharSet string

const (
	// CharSetAlphaNumeric is a character set that includes letters and numbers.
	CharSetAlphaNumeric CharSet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// CharSetAlpha is a character set that includes only letters.
	CharSetAlpha CharSet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// CharSetNumeric is a character set that includes only numbers.
	CharSetNumeric CharSet = "0123456789"
	// CharSetSpecial is a character set that includes special characters.
	CharSetSpecial CharSet = "!@#$%^&*()_+-=[]{}|;:',.<>?/"
	// CharSetAll is a character set that includes letters, numbers, and special characters.
	CharSetAll CharSet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:',.<>?/"
)

// GenerateRandStr generates a random string of the specified length using the provided character set.
// It uses the math/rand package to generate random numbers,
// so this function is not suitable for cryptographic purposes.
func GenerateRandStr(cs CharSet, strLen int) string {
	b := make([]byte, strLen)
	for i := range b {
		b[i] = cs[rand.Intn(len(cs))] // #nosec G404
	}
	return string(b)
}
