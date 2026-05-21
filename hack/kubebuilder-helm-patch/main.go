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
	pluginName := path.Base(os.Args[0])

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

	log.Println("parsed plugin request")

	response := external.PluginResponse{
		APIVersion: pluginRequest.APIVersion,
		Command:    pluginRequest.Command,
		Universe:   make(map[string]string),
	}
	switch pluginRequest.Command {
	case "init", "flags", "metadata":
		// no-op
	case "edit":
		err = validateHelmPluginCalled(pluginName, pluginRequest)
		if err == nil {
			log.Println("patching helm chart")
			response, err = patchHelmChart(pluginRequest)
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

const helmPluginName = "helm.kubebuilder.io"

func validateHelmPluginCalled(pluginName string, req *external.PluginRequest) error {
	log.Printf("validating plugin '%s' called before current plugin ('%s')", helmPluginName, pluginName)

	// not sure why but the helm plugin name is not found in
	// plugin chain, when from the documentation it seems
	// like it should. keeping this here in case that is ever
	// fixed.
	// var foundHelm bool
	// for _, plugin := range req.PluginChain {
	// 	if strings.HasPrefix(plugin, helmPluginName+"/v") {
	// 		foundHelm = true
	// 	} else if strings.HasPrefix(plugin, pluginName+"/v") {
	// 		if foundHelm {
	// 			return nil
	// 		}
	// 		return fmt.Errorf("plugin '%s' is a pre-requiste, "+
	// 			"ensure that it listed before the current plugin "+
	// 			"in the argument --plugins",
	// 			helmPluginName)
	// 	}
	// }

	_, chartExists := req.Universe[chartPath]
	_, valuesExists := req.Universe[chartValuesPath]
	if !chartExists || !valuesExists {
		return fmt.Errorf("did not find chart in universe, make sure to run the '%s' plugin before", helmPluginName)
	}
	return nil

}
