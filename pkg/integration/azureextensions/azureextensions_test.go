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
	"context"
	"testing"

	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// newFakeClient creates a kubernetes.Client with fake typed and dynamic clients.
func newFakeClient(dynamicObjs ...runtime.Object) *kubernetes.Client {
	fakeClientset := k8sfake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme, dynamicObjs...)
	return &kubernetes.Client{
		Client:        fakeClientset,
		DynamicClient: fakeDynamic,
	}
}

// newFakeDynamicClientWithResources creates a dynamic client that knows about specific GVRs.
func newFakeDynamicClientWithResources(gvrs []schema.GroupVersionResource, objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	for _, gvr := range gvrs {
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "ExtensionConfig"},
			&unstructured.Unstructured{},
		)
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "ExtensionConfigList"},
			&unstructured.UnstructuredList{},
		)
	}
	return dynamicfake.NewSimpleDynamicClient(scheme, objs...)
}

// makeExtensionConfig creates an unstructured ExtensionConfig for testing.
func makeExtensionConfig(name, namespace, apiGroup, extensionType, status, reconcileErr string, isSynced bool) *unstructured.Unstructured {
	ec := &unstructured.Unstructured{}
	ec.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   apiGroup,
		Version: APIVersion,
		Kind:    "ExtensionConfig",
	})
	ec.SetName(name)
	ec.SetNamespace(namespace)

	ec.Object["spec"] = map[string]interface{}{
		"extensionType": extensionType,
		"version":       "1.0.0",
	}
	ec.Object["status"] = map[string]interface{}{
		"status":              status,
		"reconciliationError": reconcileErr,
		"syncStatus": map[string]interface{}{
			"isSyncedWithAzure": isSynced,
			"lastSyncTime":      "2026-06-06T10:00:00Z",
		},
		"lastSuccessfulReconciledTime": "",
		"helmReleaseStatus": map[string]interface{}{
			"lastReleaseStatus":   "deployed",
			"installedChartVersion": "1.0.0",
			"lastReleaseVersion":  "1",
		},
	}
	return ec
}

func TestAnalyzeExtensionConfig_HealthyExtension(t *testing.T) {
	ec := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux", StatusInstalled, "", true)
	failures := analyzeExtensionConfig(*ec, "AKS")
	assert.Empty(t, failures, "healthy extension should produce no failures")
}

func TestAnalyzeExtensionConfig_FailedStatus(t *testing.T) {
	ec := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux", StatusFailed, "", true)
	failures := analyzeExtensionConfig(*ec, "AKS")
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0].Text, "status: Failed")
}

func TestAnalyzeExtensionConfig_PendingStatus(t *testing.T) {
	ec := makeExtensionConfig("dapr", "dapr-system", AKSAPIGroup, "microsoft.dapr", StatusPending, "", true)
	failures := analyzeExtensionConfig(*ec, "AKS")
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0].Text, "Pending")
}

func TestAnalyzeExtensionConfig_EmptyStatus(t *testing.T) {
	ec := makeExtensionConfig("test-ext", "test-ns", ArcAPIGroup, "contoso.test", "", "", true)
	failures := analyzeExtensionConfig(*ec, "Arc")
	require.NotEmpty(t, failures)
	assert.Contains(t, failures[0].Text, "empty status")
}

func TestAnalyzeExtensionConfig_ReconciliationError(t *testing.T) {
	ec := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux",
		StatusFailed, "UPGRADE FAILED: post-upgrade hooks failed: timed out waiting for the condition", true)
	failures := analyzeExtensionConfig(*ec, "AKS")

	hasReconcileErr := false
	for _, f := range failures {
		if contains(f.Text, "Reconciliation error") {
			hasReconcileErr = true
			assert.Contains(t, f.Text, "UPGRADE FAILED")
		}
	}
	assert.True(t, hasReconcileErr, "should report reconciliation error")
}

func TestAnalyzeExtensionConfig_NotSyncedWithAzure(t *testing.T) {
	ec := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux", StatusInstalled, "", false)
	failures := analyzeExtensionConfig(*ec, "AKS")

	hasSyncErr := false
	for _, f := range failures {
		if contains(f.Text, "NOT synced with Azure") {
			hasSyncErr = true
		}
	}
	assert.True(t, hasSyncErr, "should report sync failure")
}

func TestAnalyzeExtensionConfig_FailedHelmRelease(t *testing.T) {
	ec := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux", StatusFailed, "", true)
	// Override helm status to "failed"
	status := ec.Object["status"].(map[string]interface{})
	helmStatus := status["helmReleaseStatus"].(map[string]interface{})
	helmStatus["lastReleaseStatus"] = "failed"

	failures := analyzeExtensionConfig(*ec, "AKS")

	hasHelmErr := false
	for _, f := range failures {
		if contains(f.Text, "Helm release") && contains(f.Text, "failed") {
			hasHelmErr = true
		}
	}
	assert.True(t, hasHelmErr, "should report failed Helm release")
}

func TestAnalyzeExtensionConfig_NeverReconciled(t *testing.T) {
	ec := makeExtensionConfig("test", "test-ns", ArcAPIGroup, "contoso.test", StatusFailed, "some error", true)
	// lastSuccessfulReconciledTime is already "" from makeExtensionConfig
	failures := analyzeExtensionConfig(*ec, "Arc")

	hasNeverReconciled := false
	for _, f := range failures {
		if contains(f.Text, "NEVER been successfully reconciled") {
			hasNeverReconciled = true
		}
	}
	assert.True(t, hasNeverReconciled, "should report never reconciled")
}

func TestAnalyzeExtensionConfig_BothAPIGroups(t *testing.T) {
	aksEC := makeExtensionConfig("flux", "flux-system", AKSAPIGroup, "microsoft.flux", StatusFailed, "error1", true)
	arcEC := makeExtensionConfig("monitor", "azure-monitor", ArcAPIGroup, "microsoft.azuremonitor", StatusFailed, "error2", true)

	aksFailures := analyzeExtensionConfig(*aksEC, "AKS")
	arcFailures := analyzeExtensionConfig(*arcEC, "Arc")

	assert.NotEmpty(t, aksFailures, "AKS extension should have failures")
	assert.NotEmpty(t, arcFailures, "Arc extension should have failures")
	assert.Contains(t, aksFailures[0].Text, "AKS")
	assert.Contains(t, arcFailures[0].Text, "Arc")
}

func TestArcAgentsAnalyzer_NoArcNamespace(t *testing.T) {
	client := newFakeClient()
	analyzer := ArcAgentsAnalyzer{}

	a := common.Analyzer{
		Client:  client,
		Context: context.Background(),
	}

	results, err := analyzer.Analyze(a)
	assert.NoError(t, err)
	assert.Empty(t, results, "should skip when azure-arc namespace doesn't exist")
}

func TestArcAgentsAnalyzer_WithArcNamespace(t *testing.T) {
	fakeClientset := k8sfake.NewSimpleClientset()
	// Create the azure-arc namespace
	_, err := fakeClientset.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ArcNamespace},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// Register the GVRs the Arc analyzer will query
	gvrs := []schema.GroupVersionResource{
		IdentityRequestsGVR,
		ConfigSyncStatusGVRs[0],
		ConfigSyncStatusGVRs[1],
	}
	scheme := runtime.NewScheme()
	for _, gvr := range gvrs {
		gvk := schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "UnstructuredList"}
		scheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
	}
	fakeDynamic := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			IdentityRequestsGVR:  "AzureClusterIdentityRequestList",
			ConfigSyncStatusGVRs[0]: "ConfigSyncStatusList",
			ConfigSyncStatusGVRs[1]: "ConfigSyncStatusList",
		},
	)

	client := &kubernetes.Client{
		Client:        fakeClientset,
		DynamicClient: fakeDynamic,
	}

	analyzer := ArcAgentsAnalyzer{}
	a := common.Analyzer{
		Client:  client,
		Context: context.Background(),
	}

	results, err := analyzer.Analyze(a)
	assert.NoError(t, err)
	// Should detect missing secrets at minimum
	hasMissingSecret := false
	for _, r := range results {
		for _, f := range r.Error {
			if contains(f.Text, "missing") {
				hasMissingSecret = true
			}
		}
	}
	assert.True(t, hasMissingSecret, "should report missing Arc secrets")
}

func TestIntegration_GetAnalyzerName(t *testing.T) {
	a := NewAzureExtensions()
	names := a.GetAnalyzerName()
	assert.Contains(t, names, AnalyzerExtensionConfig)
	assert.Contains(t, names, AnalyzerArcAgents)
}

func TestIntegration_OwnsAnalyzer(t *testing.T) {
	a := NewAzureExtensions()
	assert.True(t, a.OwnsAnalyzer(AnalyzerExtensionConfig))
	assert.True(t, a.OwnsAnalyzer(AnalyzerArcAgents))
	assert.False(t, a.OwnsAnalyzer("Pod"))
	assert.False(t, a.OwnsAnalyzer("ScaledObject"))
}

func TestAnalyzePod_Healthy(t *testing.T) {
	pod := makePod("test-pod", "test-ns", false, false, 0)
	failures := analyzePod(pod)
	assert.Empty(t, failures)
}

func TestAnalyzePod_CrashLoopBackOff(t *testing.T) {
	pod := makePod("test-pod", "test-ns", true, false, 5)
	failures := analyzePod(pod)
	require.NotEmpty(t, failures)

	hasCrash := false
	for _, f := range failures {
		if contains(f.Text, "CrashLoopBackOff") {
			hasCrash = true
		}
	}
	assert.True(t, hasCrash)
}

func TestAnalyzePod_HighRestartCount(t *testing.T) {
	pod := makePod("test-pod", "test-ns", false, false, 10)
	failures := analyzePod(pod)

	hasRestart := false
	for _, f := range failures {
		if contains(f.Text, "restart count") {
			hasRestart = true
		}
	}
	assert.True(t, hasRestart)
}

// --- helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
