// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package utils

import (
	"math/rand"

	v1 "k8s.io/api/core/v1"
)

type LetterSet string

const (
	LowercaseAlphabeticLetterSet LetterSet = "abcdefghijklmnopqrstuvwxyz"
	UppercaseAlphabeticLetterSet LetterSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	AlphabeticLetterSet          LetterSet = LowercaseAlphabeticLetterSet + UppercaseAlphabeticLetterSet
	NumericLetterSet             LetterSet = "0123456789"
	AlphanumericLetterSet        LetterSet = AlphabeticLetterSet + NumericLetterSet
)

func GenerateRandomString(rnd *rand.Rand, n int, letterSet LetterSet) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterSet[rnd.Intn(len(letterSet))]
	}
	return string(b)
}

func GenerateRandomContainer(rnd *rand.Rand) v1.Container {
	var env []v1.EnvVar
	for i := 0; i < rnd.Intn(7); i++ {
		env = append(env, v1.EnvVar{
			Name:  GenerateRandomString(rnd, 10, UppercaseAlphabeticLetterSet),
			Value: GenerateRandomString(rnd, 15, AlphanumericLetterSet),
		})
	}

	imageRepo := GenerateRandomString(rnd, 15, LowercaseAlphabeticLetterSet)
	imageTag := GenerateRandomString(rnd, 5, LowercaseAlphabeticLetterSet)
	program := GenerateRandomString(rnd, 10, LowercaseAlphabeticLetterSet)
	param := GenerateRandomString(rnd, 5, AlphanumericLetterSet)
	return v1.Container{
		Name:    GenerateRandomString(rnd, 10, LowercaseAlphabeticLetterSet),
		Image:   imageRepo + ":" + imageTag,
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{"./" + program + ".sh", "--param", param},
		Env:     env,
	}

}
