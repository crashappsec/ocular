// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"sigs.k8s.io/kubebuilder/v4/pkg/plugin/external"
)

const (
	chartDir     = "chart/"
	templatesDir = chartDir + "templates/"

	chartPath = chartDir + "Chart.yaml"

	managerPath   = templatesDir + "manager/manager.yaml"
	configmapPath = templatesDir + "extras/controller-manager-config.yaml"
)

// replacements is the list of regexp replacements
// to apply per file.
// be sure to use $$ to escape the $ character in
// replacement text
var replacements = map[string][]replacement{
	managerPath: {
		{
			Pattern: regexp.MustCompile(`(?m)^([ ]+)labels:`),
			Replacement: "${1}labels:\n" +
				"${1}  {{- range $$key, $$val := .Values.manager.labels }}\n" +
				"${1}  {{ $$key }}: {{ $$val | quote }}\n" +
				"${1}  {{- end}}",
		},
		{
			Pattern: regexp.MustCompile(`(?m)^([ ]+)annotations:`),
			Replacement: "${1}annotations:\n" +
				"${1}  {{- range $$key, $$val := .Values.manager.annotations }}\n" +
				"${1}  {{ $$key }}: {{ $$val | quote }}\n" +
				"${1}  {{- end}}",
		},
		{
			Pattern: regexp.MustCompile(`(?m)^([ ]+)env: \[\]`),
			Replacement: "${1}env:\n" +
				"${1}  {{- if .Values.manager.env }}\n" +
				"${1}  {{- toYaml .Values.manager.env | nindent 8 }}\n" +
				"${1}  {{- else}}\n" +
				"${1}  []\n" +
				"${1}  {{- end}}",
		},
		{
			Pattern: regexp.MustCompile(`(?m)^([ ]+)volumeMounts:`),
			Replacement: "${1}volumeMounts:\n" +
				"${1}  {{- if .Values.manager.volumeMounts }}\n" +
				"${1}  {{- toYaml .Values.manager.volumeMounts | nindent 8}}\n" +
				"${1}  {{- end}}",
		},
		{
			Pattern: regexp.MustCompile(`(?m)^([ ]+)volumes:`),
			Replacement: "${1}volumes:\n" +
				"${1}  {{- if .Values.manager.volumes }}\n" +
				"${1}  {{- toYaml .Values.manager.volumes | nindent 6}}\n" +
				"${1}  {{- end}}",
		},
		{
			Pattern:     regexp.MustCompile(`\.Values`),
			Replacement: "$$values",
		},
	},
	configmapPath: {
		{
			Pattern:     regexp.MustCompile(`(?m)^([ ]+)OCULAR_SIDECAR_IMG:.*$`),
			Replacement: "${1}OCULAR_SIDECAR_IMG: {{ .Values.sidecar.image.repository }}:{{ .Values.sidecar.image.tag }}",
		},
		{
			Pattern:     regexp.MustCompile(`(?m)^([ ]+)OCULAR_SIDECAR_PULLPOLICY:.*$`),
			Replacement: "${1}OCULAR_SIDECAR_PULLPOLICY: {{ .Values.sidecar.image.pullPolicy }}",
		},
		{
			Pattern:     regexp.MustCompile(`(?m)^([ ]+)OCULAR_SCHEDULER_IMG:.*$`),
			Replacement: "${1}OCULAR_SIDECAR_IMG: {{ .Values.scheduler.image.repository }}:{{ .Values.scheduler.image.tag }}",
		},
		{
			Pattern:     regexp.MustCompile(`(?m)^([ ]+)OCULAR_SCHEDULER_PULLPOLICY:.*$`),
			Replacement: "${1}OCULAR_SIDECAR_PULLPOLICY: {{ .Values.scheduler.image.pullPolicy }}",
		},
		{
			Pattern:     regexp.MustCompile(`\.Values`),
			Replacement: "$$values",
		},
	},
}

// paddings is a map from file name
// to a [textPadding] to apply
var paddings = map[string]textPadding{
	managerPath: {
		Prefix: `
{{- $values := (tpl (.Values | toYaml) $) | fromYaml }}
{{- $values := (tpl ($values | toYaml) $) | fromYaml }}
---
`,
	},
	configmapPath: {
		Prefix: `
{{- $values := (tpl (.Values | toYaml) $) | fromYaml }}
{{- $values := (tpl ($values | toYaml) $) | fromYaml }}
---
`,
	},
}

//go:embed Chart.yaml.template
var chartTmpl string

func patchHelmChart(outputDir string, req *external.PluginRequest) (*external.PluginResponse, error) {
	resp := &external.PluginResponse{
		APIVersion: req.APIVersion,
		Command:    req.Command,
		Universe:   make(map[string]string),
	}

	if err := applyReplacements(outputDir, req, resp); err != nil {
		return resp, err
	}

	if err := applyPaddings(outputDir, req, resp); err != nil {
		return resp, err
	}

	// for now we just YQ since it preserves comments
	// eventually we should switch to a comment preserving way to set
	// values.yaml
	// mergedValues, err := mergeYAML(req.Universe[chartValuesPath], valuesYAML)
	// if err != nil {
	// return resp, err
	// }
	// resp.Universe[chartValuesPath] = mergedValues

	chart, err := templateContent("Chart.yaml", chartTmpl, map[string]string{
		"ChartVersion": strings.TrimLeft(os.Getenv("OCULAR_HELM_VERSION"), "v"),
		"AppVersion":   strings.TrimLeft(os.Getenv("OCULAR_VERSION"), "v"),
	})
	if err != nil {
		return resp, err
	}

	resp.Universe[filepath.Join(outputDir, chartPath)] = chart
	return resp, nil
}

func templateContent(name, content string, values map[string]string) (string, error) {
	t, err := template.New(name).Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", name, err)
	}
	builder := &strings.Builder{}
	err = t.Execute(builder, values)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return builder.String(), nil
}

type replacement struct {
	Pattern     *regexp.Regexp
	Replacement string
}

func (r replacement) apply(content string) (string, error) {
	if !r.Pattern.MatchString(content) {
		return content, fmt.Errorf("patch %q: pattern did not match", r.Pattern.String())
	}
	return r.Pattern.ReplaceAllString(content, r.Replacement), nil
}

func applyReplacements(outputDir string, req *external.PluginRequest, resp *external.PluginResponse) error {
	for path, rs := range replacements {
		fPath := filepath.Join(outputDir, path)
		content, err := getFileContents(fPath, req, resp)
		if err != nil {
			return err
		}
		for _, replace := range rs {
			var err error
			content, err = replace.apply(content)
			if err != nil {
				return err
			}
		}
		resp.Universe[fPath] = content
	}
	return nil
}

// textPadding adds a prefix and/or
// suffix to a file
type textPadding struct {
	Prefix string
	Suffix string
}

func (t textPadding) apply(content string) string {
	return t.Prefix + content + t.Suffix
}

func applyPaddings(outputDir string, req *external.PluginRequest, resp *external.PluginResponse) error {
	for path, padding := range paddings {
		fPath := filepath.Join(outputDir, path)
		content, err := getFileContents(fPath, req, resp)
		if err != nil {
			return err
		}

		padded := padding.apply(content)
		resp.Universe[fPath] = padded
	}
	return nil
}

func getFileContents(path string, req *external.PluginRequest, resp *external.PluginResponse) (string, error) {
	// this is done so that if an existing applyX function
	// ran before this, we use the new updated contents
	// of the file instead of the one in the request
	if path, ok := resp.Universe[path]; ok {
		return path, nil
	}

	if path, ok := req.Universe[path]; ok {
		return path, nil
	}

	return "", fmt.Errorf("file does not exist in universe: %s", path)
}
