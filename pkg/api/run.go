// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package api

import (
	"fmt"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RunEngine starts the API server with the given Gin engine.
// It will listen on the address specified in the configuration file
// values [config.State.API.Port] and will use TLS if enabled by
func RunEngine(e *gin.Engine) error {
	tlsConfig := config.State.API.TLS
	address := fmt.Sprintf("0.0.0.0:%d", config.State.API.Port)

	zap.L().Info("starting api server", zap.String("address", address))
	if !config.State.API.TLS.Enabled {
		return e.Run(address)
	}

	if tlsConfig.KeyPath == "" || tlsConfig.CertPath == "" {
		return fmt.Errorf("no certificate provided for TLS")
	}

	return e.RunTLS(address, tlsConfig.CertPath, tlsConfig.KeyPath)
}
