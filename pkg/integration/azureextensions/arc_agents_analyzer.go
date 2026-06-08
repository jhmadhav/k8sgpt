/*
Copyright 2024 The K8sGPT Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azureextensions

import (
	"fmt"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ArcAgentsAnalyzer struct{}

func (arc *ArcAgentsAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	// Only run if azure-arc namespace exists (Arc-enabled cluster)
	_, err := a.Client.GetClient().CoreV1().Namespaces().Get(a.Context, ArcNamespace, metav1.GetOptions{})
	if err != nil {
		// azure-arc namespace doesn't exist — not an Arc cluster, skip
		return a.Results, nil
	}

	var failures []common.Failure

	// Check Arc agent deployments
	deploymentFailures := checkArcDeployments(a)
	failures = append(failures, deploymentFailures...)

	// Check required secrets
	secretFailures := checkArcSecrets(a)
	failures = append(failures, secretFailures...)

	// Check identity requests
	identityFailures := checkIdentityRequests(a)
	failures = append(failures, identityFailures...)

	// Check ConfigSyncStatus for sync health
	syncFailures := checkSyncStatus(a)
	failures = append(failures, syncFailures...)

	if len(failures) > 0 {
		a.Results = append(a.Results, common.Result{
			Kind:  "AzureArcAgents",
			Name:  fmt.Sprintf("%s/arc-agents", ArcNamespace),
			Error: failures,
		})
	}

	return a.Results, nil
}

// checkArcDeployments verifies that key Arc agent deployments are healthy.
func checkArcDeployments(a common.Analyzer) []common.Failure {
	var failures []common.Failure

	for _, deployName := range ArcAgentDeployments {
		deploy, err := a.Client.GetClient().AppsV1().Deployments(ArcNamespace).Get(
			a.Context, deployName, metav1.GetOptions{})
		if err != nil {
			// Deployment not found — may not be enabled, skip silently
			continue
		}

		unavailable := deploy.Status.UnavailableReplicas
		if unavailable > 0 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Arc agent deployment '%s' has %d unavailable replicas (desired: %d, ready: %d).",
					deployName, unavailable, *deploy.Spec.Replicas, deploy.Status.ReadyReplicas),
				Sensitive: []common.Sensitive{},
			})
		}

		// Check pods for this deployment
		pods, err := a.Client.GetClient().CoreV1().Pods(ArcNamespace).List(a.Context, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", deployName),
		})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			podFailures := analyzePod(pod)
			failures = append(failures, podFailures...)

			// Tail logs for crashlooping Arc agents
			if len(podFailures) > 0 {
				logFailures := tailPodLogs(a, pod)
				failures = append(failures, logFailures...)
			}
		}
	}

	return failures
}

// checkArcSecrets verifies that required identity/certificate secrets exist.
func checkArcSecrets(a common.Analyzer) []common.Failure {
	var failures []common.Failure

	for _, secretName := range ArcRequiredSecrets {
		_, err := a.Client.GetClient().CoreV1().Secrets(ArcNamespace).Get(
			a.Context, secretName, metav1.GetOptions{})
		if err != nil {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Required Arc secret '%s' is missing in namespace '%s'. "+
					"This can cause MSI/identity failures for extensions.",
					secretName, ArcNamespace),
				Sensitive: []common.Sensitive{},
			})
		}
	}

	return failures
}

// checkIdentityRequests scans AzureClusterIdentityRequest CRs for token issues.
func checkIdentityRequests(a common.Analyzer) []common.Failure {
	var failures []common.Failure

	dynamicClient := a.Client.GetDynamicClient()
	list, err := dynamicClient.Resource(IdentityRequestsGVR).Namespace(ArcNamespace).List(
		a.Context, metav1.ListOptions{})
	if err != nil {
		return failures
	}

	for _, ir := range list.Items {
		// Check if token reference secret exists
		secretName, found, _ := unstructured.NestedString(ir.Object, "status", "tokenReference", "secretName")
		if !found || secretName == "" {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("AzureClusterIdentityRequest '%s' has no token reference — MSI token may not be provisioned.",
					ir.GetName()),
				Sensitive: []common.Sensitive{},
			})
			continue
		}

		// Verify the referenced secret exists
		_, err := a.Client.GetClient().CoreV1().Secrets(ArcNamespace).Get(
			a.Context, secretName, metav1.GetOptions{})
		if err != nil {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("AzureClusterIdentityRequest '%s' references secret '%s' which does not exist — token delivery broken.",
					ir.GetName(), secretName),
				Sensitive: []common.Sensitive{},
			})
		}
	}

	return failures
}

// checkSyncStatus examines ConfigSyncStatus CRs for ARM sync health.
func checkSyncStatus(a common.Analyzer) []common.Failure {
	var failures []common.Failure

	dynamicClient := a.Client.GetDynamicClient()

	for _, gvr := range ConfigSyncStatusGVRs {
		if !hasAnyCRDGroup(a.Client, gvr.Group) {
			continue
		}

		list, err := dynamicClient.Resource(gvr).Namespace(ArcNamespace).List(
			a.Context, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, css := range list.Items {
			syncTime, _, _ := unstructured.NestedString(css.Object, "spec", "syncTime")
			configKind, _, _ := unstructured.NestedString(css.Object, "spec", "configKind")

			if syncTime == "" {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("ConfigSyncStatus '%s' (kind: %s) has no syncTime — ARM sync may be broken.",
						css.GetName(), configKind),
					Sensitive: []common.Sensitive{},
				})
			}
		}
	}

	return failures
}
