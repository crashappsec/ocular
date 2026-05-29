// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"sigs.k8s.io/kubebuilder/v4/pkg/plugin/external"
)

func main() {
	log.Println("starting helm patch generator")

	reader := bufio.NewReader(os.Stdin)
	input, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("error will reading standard in: %s", err)
	}

	pluginRequest := &external.PluginRequest{}

	err = json.Unmarshal(input, pluginRequest)
	if err != nil {
		log.Fatalf("failed to unmarshal STDIN to JSON: %s", err)
	}

	pluginName := path.Base(os.Args[0])
	log.Println("parsed plugin request for " + pluginName)

	response := &external.PluginResponse{
		APIVersion: pluginRequest.APIVersion,
		Command:    pluginRequest.Command,
		Universe:   make(map[string]string),
	}
	switch pluginRequest.Command {
	case "init", "flags", "metadata":
		// no-op
	case "edit":
		var outputDir string
		outputDir, err = getOutputDir(pluginRequest)
		if err == nil {
			log.Println("patching helm chart")
			response, err = patchHelmChart(outputDir, pluginRequest)
		}
	default:
		err = fmt.Errorf("unknown command: %s", pluginRequest.Command)
	}
	if err != nil {
		log.Printf("error occurred in plugin: %s", err)
		response.Error = true
		response.ErrorMsgs = []string{err.Error()}
	}

	output, err := json.Marshal(response)
	if err != nil {
		log.Fatalf("unable to marshal response to JSON: %s", err)
	}

	fmt.Printf("%s", output)
}

const helmPluginName = "helm.kubebuilder.io/v2-alpha"

func getOutputDir(req *external.PluginRequest) (string, error) {
	log.Print("getting output dir from config kubebuilder helm config")
	plugins, ok := req.Config["plugins"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unable to list plugins from request")
	}

	config, ok := plugins[helmPluginName].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unable to read kubebuilder helm config '%s'", helmPluginName)
	}

	outputDir, ok := config["output"].(string)
	if !ok {
		return "", fmt.Errorf("no output dir found in config")
	}

	_, chartExists := req.Universe[path.Join(outputDir, chartPath)]
	if !chartExists {
		return "", fmt.Errorf("did not find chart in universe, make sure to run the '%s' plugin before", helmPluginName)
	}
	return outputDir, nil

}
