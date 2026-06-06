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
	"k8s.io/client-go/dynamic"
)

// getExtensionEvents looks up ExtensionEvents CRs in the same namespace as the
// failed ExtensionConfig and extracts structured diagnostic information.
func getExtensionEvents(dynamicClient dynamic.Interface, a common.Analyzer, ec unstructured.Unstructured) []common.Failure {
	var failures []common.Failure

	ns := ec.GetNamespace()
	extensionName := ec.GetName()

	list, err := dynamicClient.Resource(ExtensionEventsGVR).Namespace(ns).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return failures
	}

	for _, eventCR := range list.Items {
		// Match events to this extension by spec.extensionName
		evtExtName, _, _ := unstructured.NestedString(eventCR.Object, "spec", "extensionName")
		if evtExtName != extensionName {
			continue
		}

		operationStatus, _, _ := unstructured.NestedString(eventCR.Object, "spec", "operationStatus")
		operationName, _, _ := unstructured.NestedString(eventCR.Object, "spec", "operationName")

		// Extract individual events from the extensionEvents array
		events, found, _ := unstructured.NestedSlice(eventCR.Object, "spec", "extensionEvents")
		if !found {
			continue
		}

		for _, evt := range events {
			evtMap, ok := evt.(map[string]interface{})
			if !ok {
				continue
			}

			message, _ := evtMap["message"].(string)
			logType, _ := evtMap["logType"].(string)
			innerError, _ := evtMap["innerError"].(string)
			troubleshootLink, _ := evtMap["troubleshootLink"].(string)
			eventTime, _ := evtMap["eventTime"].(string)

			if message == "" {
				continue
			}

			text := fmt.Sprintf("[ExtensionEvent] %s (operation: %s, status: %s, time: %s): %s",
				logType, operationName, operationStatus, eventTime, message)

			if innerError != "" {
				text += fmt.Sprintf(" | InnerError: %s", innerError)
			}
			if troubleshootLink != "" {
				text += fmt.Sprintf(" | Troubleshoot: %s", troubleshootLink)
			}

			failures = append(failures, common.Failure{
				Text:      text,
				Sensitive: []common.Sensitive{},
			})
		}
	}

	return failures
}
