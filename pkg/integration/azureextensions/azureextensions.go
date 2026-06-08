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
	"os"

	"github.com/fatih/color"
	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/kubernetes"
	"github.com/spf13/viper"
)

type AzureExtensions struct{}

func NewAzureExtensions() *AzureExtensions {
	return &AzureExtensions{}
}

// Deploy is a no-op — Azure extension CRDs are managed by Azure, not by k8sgpt.
func (a *AzureExtensions) Deploy(namespace string) error {
	color.Green("Azure Extensions integration does not require deployment — CRDs are managed by Azure.")
	return nil
}

// UnDeploy is a no-op.
func (a *AzureExtensions) UnDeploy(namespace string) error {
	return nil
}

func (a *AzureExtensions) AddAnalyzer(mergedMap *map[string]common.IAnalyzer) {
	(*mergedMap)[AnalyzerExtensionConfig] = &ExtensionConfigAnalyzer{}
	(*mergedMap)[AnalyzerArcAgents] = &ArcAgentsAnalyzer{}
}

func (a *AzureExtensions) GetAnalyzerName() []string {
	return []string{
		AnalyzerExtensionConfig,
		AnalyzerArcAgents,
	}
}

func (a *AzureExtensions) GetNamespace() (string, error) {
	return "", nil
}

func (a *AzureExtensions) OwnsAnalyzer(analyzer string) bool {
	for _, name := range a.GetAnalyzerName() {
		if analyzer == name {
			return true
		}
	}
	return false
}

func (a *AzureExtensions) IsActivate() bool {
	return a.isFilterActive() && a.isDeployed()
}

func (a *AzureExtensions) isFilterActive() bool {
	activeFilters := viper.GetStringSlice("active_filters")
	for _, filter := range a.GetAnalyzerName() {
		for _, af := range activeFilters {
			if af == filter {
				return true
			}
		}
	}
	return false
}

// isDeployed checks if any Azure extension CRDs exist on the cluster.
func (a *AzureExtensions) isDeployed() bool {
	kubecontext := viper.GetString("kubecontext")
	kubeconfig := viper.GetString("kubeconfig")
	client, err := kubernetes.NewClient(kubecontext, kubeconfig)
	if err != nil {
		color.Red("Error initialising kubernetes client: %v", err)
		os.Exit(1)
	}

	return hasAnyCRDGroup(client, AKSAPIGroup) || hasAnyCRDGroup(client, ArcAPIGroup)
}

// hasAnyCRDGroup checks if any CRD from the given API group exists on the cluster.
func hasAnyCRDGroup(client *kubernetes.Client, apiGroup string) bool {
	groups, _, err := client.Client.Discovery().ServerGroupsAndResources()
	if err != nil {
		fmt.Printf("Error discovering API groups: %v\n", err)
		return false
	}
	for _, group := range groups {
		if group.Name == apiGroup {
			return true
		}
	}
	return false
}
