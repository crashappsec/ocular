// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package config

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ReadKubeConfig reads the kubeconfig file from the specified path.
// If the path is empty, it defaults to $HOME/.kube/config. If no
// file is found in either locations, it will return nil.
func ReadKubeConfig() io.Reader {
	kubeconfig := State.ClusterAccess.Kubeconfig.Path
	if kubeconfig == "" {
		kubeconfig = "~/.kube/config"
	}

	if strings.HasPrefix(kubeconfig, "~") {
		homePath, _ := os.UserHomeDir()
		kubeconfig = path.Join(homePath, strings.TrimPrefix(kubeconfig, "~"))
	}

	cleanPath := filepath.Clean(kubeconfig)

	_, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil
	}
	return file
}
