// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

// Package config provides the global configuration for Ocular.
package config

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/crashappsec/ocular/pkg/schemas"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ResourceConfig is the configuration for a resource type in Ocular.
// It contains the name of the ConfigMap that stores the resource definitions.
type ResourceConfig struct {
	ConfigMapName string `json:"configMapName" yaml:"configMapName" mapstructure:"configMapName"`
}

type SecretResourceConfig struct {
	// SecretName is the name of the secret that stores the resource definitions.
	SecretName string `json:"secretName" yaml:"secretName" mapstructure:"secretName"`
}

// Config is the structure for the global configuration file for Ocular.
// It is loaded from a config file at startup time, and values can be overridden
// by environment variables. The config file is expected to be in YAML format.
// Environment variables are expected to be prefixed with "OCULAR_", all capital
// and use underscores to separate nested keys. For example, the key
// "api.tls.enabled" can be overridden by the environment variable "OCULAR_API_TLS_ENABLED".
type Config struct {
	// Environment is the environment that Ocular is running in.
	Environment string `json:"environment" yaml:"environment"`

	// API is the configuration for the API server.
	API struct {
		// TLS is the configuration for TLS.
		TLS struct {
			// Enabled is whether TLS is enabled for the API.
			Enabled bool `json:"enabled" yaml:"enabled"`
			// CertPath is the path to the TLS certificate.
			CertPath string `json:"certpath" yaml:"certpath"`
			// KeyPath is the path to the TLS key.
			KeyPath string `json:"keypath" yaml:"keypath"`
		} `json:"tls"`
		// Port is the port that the API server will listen on.
		Port int `json:"port" yaml:"port"`
		// Host is the hostname of the API server.
		Host string `json:"host" yaml:"host"`
	} `json:"api" yaml:"api"`

	// Logging is the configuration for the logger.
	Logging struct {
		// Level is the logging level.
		Level  string `json:"level"`
		Format string `json:"format"`
	} `json:"logging" yaml:"logging"`

	// Crawlers is the configuration for crawler resources.
	Crawlers ResourceConfig `json:"crawlers" yaml:"crawlers"`

	// Downloaders is the configuration for downloader resources.
	Downloaders ResourceConfig `json:"downloaders" yaml:"downloaders"`

	// Uploaders is the configuration for uploader resources.
	Uploaders ResourceConfig `json:"uploaders" yaml:"uploaders"`

	// Profiles is the configuration for the profiles.
	Profiles ResourceConfig `json:"profiles" yaml:"profiles"`

	// Secrets is the configuration for the secrets.
	Secrets SecretResourceConfig `json:"secrets" yaml:"secrets"`

	// Extractor is the configuration for the extractor container, responsible for
	// transmitting files between scanners and uploaders.
	Extractor schemas.UserContainer `json:"extractor" yaml:"extractor"`

	// Runtime is the configuration for the runtime environment.
	Runtime struct {
		Requests struct {
			// CPU is the CPU request for the container.
			CPU string `json:"cpu" yaml:"cpu"`
			// Memory is the memory request for the container.
			Memory string `json:"memory" yaml:"memory"`
		} `json:"requests" yaml:"requests"`
		Limits struct {
			// CPU is the CPU limit for the container.
			CPU string `json:"cpu" yaml:"cpu"`
			// Memory is the memory limit for the container.
			Memory string `json:"memory" yaml:"memory"`
		}
		Labels map[string]string `json:"labels" yaml:"labels" mapstructure:"labels"`
		// ImagePullSecrets is a list of image pull secrets that will be attached to
		// all jobs created by Ocular.
		ImagePullSecrets []string `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty,flow" mapstructure:"imagePullSecrets"`

		// JobTTL is the time to live for jobs created by Ocular.
		JobTTL time.Duration `json:"jobTTL" yaml:"jobTTL" mapstructure:"jobTTL"`

		// UploadersServiceAccount is the service account that will be used by the uploader pods of a pipeline.
		UploadersServiceAccount string `json:"uploadersServiceAccount,omitempty" yaml:"uploadersServiceAccount,omitempty" mapstructure:"uploadersServiceAccount"`

		// ScannersServiceAccount is the service account that will be used by the scanner pods of a pipeline.
		// This will also be used by the downloader in the scanner job.
		ScannersServiceAccount string `json:"scannersServiceAccount,omitempty" yaml:"scannersServiceAccount,omitempty" mapstructure:"scannersServiceAccount"`

		// CrawlersServiceAccount is the service account that will be used by the search jobs.
		CrawlersServiceAccount string `json:"crawlersServiceAccount,omitempty" yaml:"crawlersServiceAccount,omitempty" mapstructure:"crawlersServiceAccount"`
	} `json:"runtime" yaml:"runtime" mapstructure:"runtime"`

	// ClusterAccess is the configuration for the cluster contexts
	// that are used by Ocular. A 'ClusterAccess' represents a connection
	// to a cluster (via client-go's ClientSet) and a namespace.
	// Contexts are read both from a kubeconfig file and from the
	// mounted service account token in the pod.
	ClusterAccess struct {
		CheckValidity  bool `json:"checkValidity" yaml:"checkValidity"`
		ServiceAccount struct {
			Enabled   bool   `json:"enabled" yaml:"enabled"`
			Namespace string `json:"namespace" yaml:"namespace"`
		} `json:"serviceAccount" yaml:"serviceAccount"`

		Kubeconfig struct {
			Path string `json:"path" yaml:"path"`
		} `json:"kubeconfig" yaml:"kubeconfig"`
	} `json:"clusterAccess" yaml:"clusterAccess"`
}

// State is the global configuration state for Ocular.
var State Config

func Init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/ocular/")
	viper.AddConfigPath("$HOME/.ocular")
	viper.AddConfigPath(".")

	if configPath, exists := os.LookupEnv("OCULAR_CONFIG_PATH"); exists {
		// If the OCULAR_CONFIG_PATH environment variable is set, add it as a config path.
		viper.AddConfigPath(configPath)
	}

	// have to use something that will most likely not be a
	// key anywhere in the config file, so that we can
	// use it as a delimiter for the viper keys.
	// By default viper uses "." as a delimiter, which is not
	// suitable for Ocular, as when we have labels (or annotations)
	// that we want to parse into a map, it creates sub-maps for each ".", i.e.
	// "my.custom.label: value" becomes {"my": {"custom": {"label": "value"}}}
	delimiter := "%"
	viper.SetOptions(viper.KeyDelimiter(delimiter))

	viper.SetEnvPrefix("ocular")
	viper.SetEnvKeyReplacer(strings.NewReplacer(delimiter, "_"))
	viper.SetDefault("ClusterAccess%Kubeconfig%Path", "~/.kube/config")
	// only errors if no input is given, so can ignore
	_ = viper.BindEnv("ClusterAccess%Kubeconfig%Path", "KUBECONFIG")
	viper.SetDefault("API%TLS%Enabled", false)
	viper.SetDefault("API%Port", "3001")
	viper.SetDefault("Environment", "production")
	viper.SetDefault("API%Host", "ocular-api-server.ocular.svc.cluster.local")

	viper.SetDefault("Crawlers%ConfigMapName", "ocular-crawlers")

	viper.SetDefault("Downloaders%ConfigMapName", "ocular-downloaders")

	viper.SetDefault("Extractor%Image", "ocular-extractor")

	viper.SetDefault("Logging%Level", "info")
	viper.SetDefault("Logging%Format", "json")

	viper.SetDefault("Profiles%ConfigMapName", "ocular-profiles")

	viper.SetDefault("Runtime%Requests%CPU", "100m")
	viper.SetDefault("Runtime%Requests%Memory", "128Mi")
	viper.SetDefault("Runtime%JobTTL", "3m")
	viper.SetDefault("Runtime%UploadersServiceAccount", "")
	viper.SetDefault("Runtime%ScannersServiceAccount", "")
	viper.SetDefault("Runtime%CrawlersServiceAccount", "")

	viper.SetDefault("Secrets%SecretName", "ocular-secrets")

	viper.SetDefault("Uploaders%ConfigMapName", "ocular-uploaders")

	err := viper.ReadInConfig()
	if err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			zap.L().Error("error reading config", zap.Error(err))
			return
		} else if err != nil {
			zap.L().Info("config file not found, using defaults")
		}
	}
	viper.AutomaticEnv()

	err = viper.Unmarshal(
		&State,
		viper.DecodeHook(
			func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
				// Custom decode hook for time.Duration
				if t == reflect.TypeOf(time.Duration(0)) {
					if f.Kind() == reflect.String {
						return time.ParseDuration(data.(string))
					}
				}

				return data, nil
			},
		),
	)
	if err != nil {
		zap.L().Error("error unmarshalling config", zap.Error(err))
	}
	InitLogger(State.Logging.Level, State.Logging.Format,
		zap.Any("build_metadata", map[string]string{
			"version":    Version,
			"build_time": BuildTime,
			"commit":     Commit,
		}))
	InitEnv()
}

func WriteConfig(w io.Writer) error {
	if err := viper.WriteConfigTo(w); err != nil {
		return err
	}
	return nil
}
