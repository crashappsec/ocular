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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/crashappsec/ocular/test/utils"
)

// namespace where the project is deployed in
const pipelineNamespace = "e2e-test-pipeline"

var _ = Describe("Pipeline", Ordered, func() {

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("creating pipeline E2E namespace")
		cmd = exec.Command("kubectl", "create", "ns", pipelineNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("labeling the pipeline namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", pipelineNamespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label pipeline namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy-e2e-test")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		By("ensuring the webhook service is healthly")
		Expect(CreateWebhookServiceCheckPod()).To(Succeed(), "Failed to create webhook service check pod")

		By("waiting for webhook check pod to complete")
		Eventually(VerifyWebhookServiceCheck, 5*time.Minute).Should(Succeed())

		By("deploying the pipeline test resources")
		cmd = exec.Command("make", "run-e2e-test-pipeline")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the pipeline test resources")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the await-webhook pod")
		cmd := exec.Command("kubectl", "delete", "pod", awaitWebhookPodName, "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "stop-e2e-test-pipeline", "undeploy-e2e-test")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager & pipeline namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace, pipelineNamespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", "-l", "control-plane=controller-manager", "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching scan pod description")
			cmd = exec.Command("kubectl", "describe", "pod", "e2e-test-scan", "-n", pipelineNamespace)
			scanPodDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", scanPodDescription)
			} else {
				fmt.Println("Failed to describe scan pod")
			}

			By("Fetching upload pod description")
			cmd = exec.Command("kubectl", "describe", "pod", "e2e-test-upload", "-n", pipelineNamespace)
			uploadPodDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", uploadPodDescription)
			} else {
				fmt.Println("Failed to describe upload pod")
			}

			By("Fetching scan and upload pod logs")
			cmd = exec.Command("kubectl", "logs", "-l", "ocular.crashoverride.run/pipeline=e2e-test",
				"--all-containers", "-n", pipelineNamespace, "--tail", "-1")
			pipelineLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Pipeline logs:\n %s", pipelineLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get pipeline logs: %s", err)
			}
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second * 5)

	Context("Pipeline", func() {
		It("should run successfully", func() {
			By("validating that the created pipeliene finishes with Success")
			verifyControllerUp := func(g Gomega) {
				// Validate the pipline's status
				cmd := exec.Command("kubectl", "get",
					"pipeline", "e2e-test", "-o", "jsonpath={.status.phase}",
					"-n", pipelineNamespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "Incorrect pipeline pod status")

			}
			Eventually(verifyControllerUp).Should(Succeed())
		})
	})
})
