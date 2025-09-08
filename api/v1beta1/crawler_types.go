// Copyright (C) 2025 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package v1beta1

import (
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CrawlerRunRef = ParameterizedRunRef

type CrawlerStatus struct {
	// Conditions is a list of conditions that the uploader is in.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"The latest available observations of a Uploader's current state."`
	// Valid indicates whether the uploader is valid.
	// +optional
	Valid bool `json:"valid" description:"Whether or not the uploader is valid."`
}

// CrawlerSpec defines the desired state of Crawler
type CrawlerSpec struct {
	// Container is the container that will be run to download the target.
	// It must be a valid [v1.Container] that can be run in a Kubernetes pod.
	// +required
	Container v1.Container `json:"container" yaml:"container" description:"The container that will be run to enumerate targets and create Pipelines."`

	// Volumes is a list of volumes that will be appended to the [k8s.io/api/core/v1.PodSpec]
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty" yaml:"volumes,omitempty" description:"A list of volumes that will be mounted into the downloader container. This is useful for sharing data between downloaders or for providing configuration files."`

	// Parameters is a map of parameters that can be used to define additional parameters
	// that the crawler can use. The keys are the parameter names, and the values
	// are the definitions of the parameters. The uploader can use these parameters
	// to customize its behavior. The parameters can be used in the uploader's command line
	// arguments, environment variables, or any other way that the uploader supports.
	// +optional
	Parameters map[string]ParameterDefinition `json:"parameters,omitempty" yaml:"parameters,omitempty" description:"Parameters used to define additional parameters."`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Crawler is the Schema for the crawlers API
type Crawler struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Crawler
	// +required
	Spec CrawlerSpec `json:"spec"`

	// status defines the observed state of Crawler
	// +optional
	Status CrawlerStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// CrawlerList contains a list of Crawler
type CrawlerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Crawler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Crawler{}, &CrawlerList{})
}

var ErrParameter = errors.New("invalid parameter")

func ValidateCrawlerParameters(crawler Crawler, parameters map[string]string) error {
	for name, def := range crawler.Spec.Parameters {
		paramValue, exists := parameters[name]
		if def.Required && (!exists || paramValue == "") {
			return fmt.Errorf("%w: param '%s' is required but not provided", ErrParameter, name)
		}
	}
	for name := range parameters {
		_, exists := crawler.Spec.Parameters[name]
		if !exists {
			return fmt.Errorf("%w: param '%s' is not defined in crawler spec", ErrParameter, name)
		}
	}
	return nil
}
