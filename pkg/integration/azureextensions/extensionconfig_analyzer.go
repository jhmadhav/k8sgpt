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
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ExtensionConfigAnalyzer struct{}

func (e *ExtensionConfigAnalyzer) Analyze(a common.Analyzer) ([]common.Result, error) {
	dynamicClient := a.Client.GetDynamicClient()

	for _, gvr := range ExtensionConfigGVRs {
		// Check if this CRD group exists on the cluster
		if !hasAnyCRDGroup(a.Client, gvr.Group) {
			continue
		}

		list, err := dynamicClient.Resource(gvr).Namespace(a.Namespace).List(a.Context, metav1.ListOptions{
			LabelSelector: a.LabelSelector,
		})
		if err != nil {
			continue
		}

		source := "Arc"
		if gvr.Group == AKSAPIGroup {
			source = "AKS"
		}

		for _, ec := range list.Items {
			failures := analyzeExtensionConfig(ec, source)

			// Enrich with ExtensionEvents from the same namespace
			eventFailures := getExtensionEvents(dynamicClient, a, ec)
			failures = append(failures, eventFailures...)

			// Enrich with pod-level diagnostics in the extension namespace
			podFailures := getPodDiagnostics(a, ec)
			failures = append(failures, podFailures...)

			if len(failures) > 0 {
				result := common.Result{
					Kind:  fmt.Sprintf("AzureExtensionConfig(%s)", source),
					Name:  fmt.Sprintf("%s/%s", ec.GetNamespace(), ec.GetName()),
					Error: failures,
				}
				a.Results = append(a.Results, result)
			}
		}
	}

	return a.Results, nil
}

// analyzeExtensionConfig checks the ExtensionConfig CR for common failure patterns.
func analyzeExtensionConfig(ec unstructured.Unstructured, source string) []common.Failure {
	var failures []common.Failure

	extensionType, _, _ := unstructured.NestedString(ec.Object, "spec", "extensionType")
	version, _, _ := unstructured.NestedString(ec.Object, "spec", "version")
	name := ec.GetName()
	ns := ec.GetNamespace()

	// Check status.status
	status, _, _ := unstructured.NestedString(ec.Object, "status", "status")
	switch status {
	case StatusFailed:
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("%s extension '%s' (type: %s, version: %s) in namespace '%s' has status: Failed.",
				source, name, extensionType, version, ns),
			Sensitive: []common.Sensitive{},
		})
	case StatusPending:
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("%s extension '%s' (type: %s) in namespace '%s' is stuck in Pending status.",
				source, name, extensionType, ns),
			Sensitive: []common.Sensitive{},
		})
	case "":
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("%s extension '%s' (type: %s) in namespace '%s' has empty status — may not have been reconciled.",
				source, name, extensionType, ns),
			Sensitive: []common.Sensitive{},
		})
	}

	// Check status.reconciliationError
	reconcileErr, found, _ := unstructured.NestedString(ec.Object, "status", "reconciliationError")
	if found && reconcileErr != "" {
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Reconciliation error for extension '%s': %s", name, reconcileErr),
			Sensitive: []common.Sensitive{
				{
					Unmasked: reconcileErr,
					Masked:   util.MaskString(reconcileErr),
				},
			},
		})
	}

	// Check status.extensionState
	extensionState, found, _ := unstructured.NestedString(ec.Object, "status", "extensionState")
	if found && extensionState != "" && extensionState != StatusInstalled {
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Extension '%s' extensionState is '%s' (expected: Installed).", name, extensionState),
			Sensitive: []common.Sensitive{},
		})
	}

	// Check status.helmReleaseStatus.lastReleaseStatus
	helmStatus, found, _ := unstructured.NestedString(ec.Object, "status", "helmReleaseStatus", "lastReleaseStatus")
	if found && helmStatus != "" && helmStatus != "deployed" {
		chartVersion, _, _ := unstructured.NestedString(ec.Object, "status", "helmReleaseStatus", "installedChartVersion")
		releaseVersion, _, _ := unstructured.NestedString(ec.Object, "status", "helmReleaseStatus", "lastReleaseVersion")
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Helm release for extension '%s' has status '%s' (chart: %s, revision: %s).",
				name, helmStatus, chartVersion, releaseVersion),
			Sensitive: []common.Sensitive{},
		})
	}

	// Check status.syncStatus.isSyncedWithAzure
	isSynced, found, _ := unstructured.NestedBool(ec.Object, "status", "syncStatus", "isSyncedWithAzure")
	if found && !isSynced {
		lastSyncTime, _, _ := unstructured.NestedString(ec.Object, "status", "syncStatus", "lastSyncTime")
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Extension '%s' is NOT synced with Azure (last sync: %s). The cluster may be disconnected or the sync pipeline is broken.",
				name, lastSyncTime),
			Sensitive: []common.Sensitive{},
		})
	}

	// Check status.lastSuccessfulReconciledTime — empty means never succeeded
	lastSuccess, found, _ := unstructured.NestedString(ec.Object, "status", "lastSuccessfulReconciledTime")
	if found && lastSuccess == "" && status == StatusFailed {
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Extension '%s' has NEVER been successfully reconciled — initial installation likely failed.", name),
			Sensitive: []common.Sensitive{},
		})
	}

	// Check status.message for additional context
	message, found, _ := unstructured.NestedString(ec.Object, "status", "message")
	if found && message != "" && status != StatusInstalled {
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Extension '%s' status message: %s", name, message),
			Sensitive: []common.Sensitive{
				{
					Unmasked: message,
					Masked:   util.MaskString(message),
				},
			},
		})
	}

	return failures
}
