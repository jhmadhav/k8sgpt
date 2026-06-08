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

import "k8s.io/apimachinery/pkg/runtime/schema"

// API groups for Azure K8s Extensions CRDs
const (
	// AKS clusters use this API group for extension configs
	AKSAPIGroup = "aks.clusterconfig.azure.com"
	// Arc-enabled clusters use this API group for extension configs
	ArcAPIGroup = "clusterconfig.azure.com"
	// Arc resource group for connectedclusters/provisionedclusters
	ArcResourceGroup = "arc.azure.com"
	// API version for all extension CRDs
	APIVersion = "v1beta1"
	// Arc agents namespace and Helm release name
	ArcNamespace   = "azure-arc"
	ArcReleaseName = "azure-arc-release"
)

// GVRs for ExtensionConfig on both AKS and Arc
var ExtensionConfigGVRs = []schema.GroupVersionResource{
	{Group: AKSAPIGroup, Version: APIVersion, Resource: "extensionconfigs"},
	{Group: ArcAPIGroup, Version: APIVersion, Resource: "extensionconfigs"},
}

// GVR for ExtensionEvents (currently only on Arc API group)
var ExtensionEventsGVR = schema.GroupVersionResource{
	Group: ArcAPIGroup, Version: APIVersion, Resource: "extensionevents",
}

// GVRs for ConfigSyncStatus (follows same migration pattern as ExtensionConfig)
var ConfigSyncStatusGVRs = []schema.GroupVersionResource{
	{Group: AKSAPIGroup, Version: APIVersion, Resource: "configsyncstatuses"},
	{Group: ArcAPIGroup, Version: APIVersion, Resource: "configsyncstatuses"},
}

// GVRs for Arc-only diagnostics CRDs
var (
	HealthStatesGVR = schema.GroupVersionResource{
		Group: ArcAPIGroup, Version: APIVersion, Resource: "healthstates",
	}
	ArcCertificatesGVR = schema.GroupVersionResource{
		Group: ArcAPIGroup, Version: APIVersion, Resource: "arccertificates",
	}
	IdentityRequestsGVR = schema.GroupVersionResource{
		Group: ArcAPIGroup, Version: APIVersion, Resource: "azureclusteridentityrequests",
	}
	ExtensionIdentitiesGVR = schema.GroupVersionResource{
		Group: ArcAPIGroup, Version: APIVersion, Resource: "azureextensionidentities",
	}
)

// Analyzer names (used as filter keys)
const (
	AnalyzerExtensionConfig = "AzureExtensionConfig"
	AnalyzerArcAgents       = "AzureArcAgents"
)

// Known extension status values
const (
	StatusInstalled = "Installed"
	StatusFailed    = "Failed"
	StatusPending   = "Pending"
	StatusDeleting  = "Deleting"
)

// Arc agent deployments to check in azure-arc namespace
var ArcAgentDeployments = []string{
	"extension-manager",
	"config-agent",
	"cluster-metadata-operator",
	"clusteridentityoperator",
	"clusterconnect-agent",
	"resource-sync-agent",
	"kube-aad-proxy",
	"extension-events-collector",
	"flux-logs-agent",
	"metrics-agent",
}

// Required secrets in azure-arc namespace
var ArcRequiredSecrets = []string{
	"azure-identity-certificate",
	"kube-aad-proxy-certificate",
}
