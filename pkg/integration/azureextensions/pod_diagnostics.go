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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	maxLogLines = 50
)

// getPodDiagnostics checks pods in the extension's namespace for common failure patterns
// and tails recent logs for error context.
func getPodDiagnostics(a common.Analyzer, ec unstructured.Unstructured) []common.Failure {
	var failures []common.Failure

	ns := ec.GetNamespace()
	if ns == "" {
		return failures
	}

	pods, err := a.Client.GetClient().CoreV1().Pods(ns).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return failures
	}

	for _, pod := range pods.Items {
		podFailures := analyzePod(pod)
		failures = append(failures, podFailures...)

		// If pod is crashlooping or has errors, tail logs
		if len(podFailures) > 0 {
			logFailures := tailPodLogs(a, pod)
			failures = append(failures, logFailures...)
		}
	}

	return failures
}

// analyzePod checks a single pod for common failure states.
func analyzePod(pod v1.Pod) []common.Failure {
	var failures []common.Failure

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError":
				msg := cs.State.Waiting.Message
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Pod '%s/%s' container '%s': %s — %s",
						pod.Namespace, pod.Name, cs.Name, reason, msg),
					Sensitive: []common.Sensitive{},
				})
			}
		}

		if cs.State.Terminated != nil {
			reason := cs.State.Terminated.Reason
			exitCode := cs.State.Terminated.ExitCode
			if exitCode != 0 {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Pod '%s/%s' container '%s': terminated with %s (exit code: %d)",
						pod.Namespace, pod.Name, cs.Name, reason, exitCode),
					Sensitive: []common.Sensitive{},
				})
			}
		}

		if cs.RestartCount > 3 {
			failures = append(failures, common.Failure{
				Text: fmt.Sprintf("Pod '%s/%s' container '%s': high restart count (%d restarts)",
					pod.Namespace, pod.Name, cs.Name, cs.RestartCount),
				Sensitive: []common.Sensitive{},
			})
		}
	}

	// Check for pending pods with scheduling issues
	if pod.Status.Phase == v1.PodPending {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodScheduled && condition.Status == v1.ConditionFalse {
				failures = append(failures, common.Failure{
					Text: fmt.Sprintf("Pod '%s/%s' is Pending — cannot be scheduled: %s",
						pod.Namespace, pod.Name, condition.Message),
					Sensitive: []common.Sensitive{},
				})
			}
		}
	}

	return failures
}

// tailPodLogs tails the last N lines of a pod's first container logs and extracts error patterns.
func tailPodLogs(a common.Analyzer, pod v1.Pod) []common.Failure {
	var failures []common.Failure

	if len(pod.Spec.Containers) == 0 {
		return failures
	}

	tailLines := int64(maxLogLines)
	req := a.Client.GetClient().CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		TailLines: &tailLines,
	})

	stream, err := req.Stream(a.Context)
	if err != nil {
		return failures
	}
	defer stream.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, stream)
	if err != nil {
		return failures
	}

	// Scan for error patterns
	var errorLines []string
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "fatal") ||
			strings.Contains(lower, "panic") ||
			strings.Contains(lower, "exception") ||
			strings.Contains(lower, "failed") {
			errorLines = append(errorLines, line)
		}
	}

	if len(errorLines) > 0 {
		// Limit to last 5 error lines to avoid overwhelming the LLM
		if len(errorLines) > 5 {
			errorLines = errorLines[len(errorLines)-5:]
		}
		failures = append(failures, common.Failure{
			Text: fmt.Sprintf("Pod '%s/%s' recent log errors:\n%s",
				pod.Namespace, pod.Name, strings.Join(errorLines, "\n")),
			Sensitive: []common.Sensitive{},
		})
	}

	return failures
}
