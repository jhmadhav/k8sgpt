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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getHelmDiagnostics checks Helm release secrets in the extension namespace
// to find failed releases and their error messages.
func getHelmDiagnostics(a common.Analyzer, namespace, extensionName string) []common.Failure {
	var failures []common.Failure

	// Helm stores release info in secrets with label: owner=helm
	secrets, err := a.Client.GetClient().CoreV1().Secrets(namespace).List(a.Context, metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err != nil {
		return failures
	}

	for _, secret := range secrets.Items {
		releaseName := secret.Labels["name"]
		status := secret.Labels["status"]
		version := secret.Labels["version"]

		if status == "failed" || status == "pending-install" || status == "pending-upgrade" {
			text := fmt.Sprintf("[HelmRelease] Release '%s' (revision %s) has status '%s'.",
				releaseName, version, status)

			// Try to extract the error description from the release data
			if releaseData, ok := secret.Data["release"]; ok {
				desc := extractHelmDescription(releaseData)
				if desc != "" {
					text += fmt.Sprintf(" Description: %s", desc)
				}
			}

			failures = append(failures, common.Failure{
				Text:      text,
				Sensitive: []common.Sensitive{},
			})
		}
	}

	return failures
}

// extractHelmDescription tries to extract the description field from a Helm release secret.
// The data is base64 -> gzip -> JSON, but we just search for common error patterns.
func extractHelmDescription(data []byte) string {
	// Helm release data is double-encoded, try simple JSON parse first
	var release map[string]interface{}
	if err := json.Unmarshal(data, &release); err == nil {
		if info, ok := release["info"].(map[string]interface{}); ok {
			if desc, ok := info["description"].(string); ok {
				return desc
			}
		}
	}
	return ""
}

// getK8sEvents collects recent Warning events from the extension namespace.
func getK8sEvents(a common.Analyzer, namespace string) []common.Failure {
	var failures []common.Failure

	events, err := a.Client.GetClient().CoreV1().Events(namespace).List(a.Context, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		return failures
	}

	// Collect unique warning events (dedup by reason+message)
	seen := make(map[string]bool)
	var warnings []string

	for _, event := range events.Items {
		key := fmt.Sprintf("%s/%s", event.Reason, event.InvolvedObject.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		count := event.Count
		if count == 0 {
			count = 1
		}
		warnings = append(warnings, fmt.Sprintf("%s on %s/%s: %s (x%d)",
			event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Name,
			event.Message, count))
	}

	if len(warnings) > 0 {
		// Limit to 10 most relevant warnings
		if len(warnings) > 10 {
			warnings = warnings[len(warnings)-10:]
		}
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("[K8sEvents] %d warning events in namespace '%s':\n%s",
				len(warnings), namespace, strings.Join(warnings, "\n")),
			Sensitive: []common.Sensitive{},
		})
	}

	return failures
}

// getDeploymentDiagnostics checks deployments in the extension namespace for issues.
func getDeploymentDiagnostics(a common.Analyzer, namespace string) []common.Failure {
	var failures []common.Failure

	deployments, err := a.Client.GetClient().AppsV1().Deployments(namespace).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return failures
	}

	for _, deploy := range deployments.Items {
		if deploy.Status.UnavailableReplicas > 0 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("[Deployment] '%s/%s' has %d unavailable replicas (desired: %d, ready: %d, updated: %d).",
					namespace, deploy.Name,
					deploy.Status.UnavailableReplicas,
					*deploy.Spec.Replicas,
					deploy.Status.ReadyReplicas,
					deploy.Status.UpdatedReplicas),
				Sensitive: []common.Sensitive{},
			})
		}

		// Check for deployment conditions
		for _, cond := range deploy.Status.Conditions {
			if cond.Type == "Available" && cond.Status == v1.ConditionFalse {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("[Deployment] '%s/%s' is not Available: %s — %s",
						namespace, deploy.Name, cond.Reason, cond.Message),
					Sensitive: []common.Sensitive{},
				})
			}
			if cond.Type == "Progressing" && cond.Status == v1.ConditionFalse {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("[Deployment] '%s/%s' is not Progressing: %s — %s",
						namespace, deploy.Name, cond.Reason, cond.Message),
					Sensitive: []common.Sensitive{},
				})
			}
		}
	}

	return failures
}

// getJobDiagnostics checks for failed Jobs (Helm hooks) in the extension namespace.
func getJobDiagnostics(a common.Analyzer, namespace string) []common.Failure {
	var failures []common.Failure

	jobs, err := a.Client.GetClient().BatchV1().Jobs(namespace).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return failures
	}

	for _, job := range jobs.Items {
		for _, cond := range job.Status.Conditions {
			if cond.Type == "Failed" && cond.Status == v1.ConditionTrue {
				// Check if it's a Helm hook
				hookType := ""
				if ann, ok := job.Annotations["helm.sh/hook"]; ok {
					hookType = fmt.Sprintf(" (Helm hook: %s)", ann)
				}
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("[Job] '%s/%s'%s failed: %s — %s",
						namespace, job.Name, hookType, cond.Reason, cond.Message),
					Sensitive: []common.Sensitive{},
				})

				// Tail logs from the failed job's pods
				jobPods, _ := a.Client.GetClient().CoreV1().Pods(namespace).List(a.Context, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
				})
				if jobPods != nil {
					for _, pod := range jobPods.Items {
						logFailures := tailPodLogs(a, pod)
						failures = append(failures, logFailures...)
					}
				}
			}
		}
	}

	return failures
}
