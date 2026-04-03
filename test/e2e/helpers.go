//go:build e2e

// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.
package e2e

import (
	"fmt"
	"os/exec"

	"github.com/crashappsec/ocular/test/utils"
	. "github.com/onsi/gomega"
)

const curlWebhookHealthSpec = `
{
  "spec": {
    "containers": [
      {
        "name": "curl",
        "image": "curlimages/curl:latest",
        "command": [
          "/bin/sh",
          "-c"
        ],
        "args": [
          "for i in $(seq 1 30); do curl -v -k https://%s.%s.svc.cluster.local/healthz && exit 0 || sleep 2; done; exit 1"
        ],
        "securityContext": {
          "readOnlyRootFilesystem": true,
          "allowPrivilegeEscalation": false,
          "capabilities": {
            "drop": [
              "ALL"
            ]
          },
          "runAsNonRoot": true,
          "runAsUser": 1000,
          "seccompProfile": {
            "type": "RuntimeDefault"
          }
        }
      }
    ]
  }
}
`

const awaitWebhookPodName = "await-webhook-service"

// webhookServiceName is the name of the webhook service of the project
const webhookServiceName = "ocular-webhook-service"

func CreateWebhookServiceCheckPod() error {
	cmd := exec.Command("kubectl", "run", awaitWebhookPodName, "--restart=Never",
		"--namespace", namespace,
		"--image=curlimages/curl:latest",
		"--overrides",
		fmt.Sprintf(curlWebhookHealthSpec, webhookServiceName, namespace))
	_, err := utils.Run(cmd)
	return err
}

func VerifyWebhookServiceCheck(g Gomega) {
	cmd := exec.Command("kubectl", "get", "pods", awaitWebhookPodName,
		"-o", "jsonpath={.status.phase}",
		"-n", namespace)
	output, err := utils.Run(cmd)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
}
