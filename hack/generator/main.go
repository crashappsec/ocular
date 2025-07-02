// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Copyright 2025 (C)
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crashappsec/ocular/internal/config"
	"github.com/crashappsec/ocular/internal/utilities"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

var (
	outputFilename  string
	configType      string
	configFormat    string
	additionalFiles []string
)

const (
	ConfigTypeAPI     = "api-config"
	ConfigTypeOpenAPI = "open-api"

	ConfigFormatYAML = "yaml"
	ConfigFormatJSON = "json"
)

var validConfigTypes = []string{ConfigTypeAPI, ConfigTypeOpenAPI}

var validConfigFormats = []string{ConfigFormatYAML, ConfigFormatJSON}

func init() {
	flag.StringVar(
		&outputFilename,
		"output",
		"",
		"The name of the output file, if omitted or set to '-' the output will be printed to stdout",
	)
	flag.StringVar(
		&configType,
		"type",
		ConfigTypeAPI,
		fmt.Sprintf("The type of configuration to generate, must be one of [%s]. Defaults to '%s'.",
			strings.Join(validConfigTypes, ", "), ConfigTypeAPI),
	)
	flag.StringVar(
		&configFormat,
		"format",
		ConfigFormatYAML,
		fmt.Sprintf(
			"The format of configuration to generate, must be one of [%s]. Defaults to '%s'.",
			strings.Join(validConfigFormats, ", "),
			ConfigFormatYAML,
		),
	)
	config.InitLogger(
		os.Getenv("OCULAR_LOGGING_LEVEL"),
		os.Getenv("OCULAR_LOGGING_FORMAT"))
}

func main() {
	flag.Parse()

	for _, validType := range validConfigTypes {
		if configType == validType {
			zap.L().Info("Generating configuration", zap.String("type", configType))
			break
		}
	}

	additionalFiles = flag.Args()

	var (
		w   = os.Stdout
		err error
	)
	if outputFilename != "" && outputFilename != "-" {
		zap.L().Info("Writing output file", zap.String("filename", outputFilename))
		w, err = os.Create(filepath.Clean(outputFilename))
		if err != nil {
			zap.L().
				Fatal("Failed to create output file", zap.String("filename", outputFilename), zap.Error(err))
		}
	} else {
		zap.L().Info("Writing output to stdout")
	}

	defer utilities.CloseAndLog(w)

	switch configType {
	case ConfigTypeAPI:
		zap.L().Info("Generating API configuration")
		err = config.WriteConfig(w)
		if err != nil {
			zap.L().Fatal("Failed to write config", zap.Error(err))
		}
	case ConfigTypeOpenAPI:
		zap.L().Info("Generating OpenAPI configuration")
		err = WriteOpenAPI(w, configFormat)
	default:
		zap.L().Fatal("Invalid configuration type", zap.String("type", configType))
	}

	if err != nil {
		zap.L().Fatal("Failed to write config", zap.Error(err))
	}

	if len(additionalFiles) == 0 {
		zap.L().Info("No additional files specified, skipping")
		return
	}

	var merr *multierror.Error
	for _, file := range additionalFiles {
		f, err := os.Open(filepath.Clean(file))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			merr = multierror.Append(merr, err)
			continue
		}

		_, err = f.WriteTo(w)
		utilities.CloseAndLog(f)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
	}

	if err = merr.ErrorOrNil(); err != nil {
		zap.L().Fatal("Failed to write additional files", zap.Error(err))
	}

	zap.L().Info("Finished writing config")
}

func GetEnvDefault(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}
