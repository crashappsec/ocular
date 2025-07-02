// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package config

import (
	"go.uber.org/zap"
)

// InitLogger initializes the logger for the application.
// It sets the logging level and format based on the configuration.
// It also replaces the global logger with the new logger.
func InitLogger(level, format string, globalFields ...zap.Field) {
	if format == "" {
		format = "json"
	}
	var opts []zap.Option

	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		zap.L().Sugar().Error("error parsing log level", zap.Error(err))
		lvl = zap.NewAtomicLevel()
	}

	var cfg zap.Config
	if IsEnvironmentIn(EnvProduction, EnvStaging) {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.Level = lvl
	cfg.Encoding = format

	logger := zap.Must(cfg.Build(opts...)).With(globalFields...)
	logger.Info("logger initialized")

	zap.ReplaceGlobals(logger)
}
