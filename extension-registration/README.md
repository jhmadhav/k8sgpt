# Extension Registration: microsoft.extensions.k8sgpt

## Overview

AI-powered diagnostics for Azure Kubernetes Extensions using k8sgpt.

## Registration Artifacts

| File | Purpose |
|------|---------|
| `type-registration.json` | Extension type registration — defines the extension type, supported clusters, scope, billing |
| `package-config.json` | Package configuration — supported cluster types, config schema, reconciler settings |
| `version-config.json` | Version configuration — Helm chart location, release train, regions |

## Extension Details

| Field | Value |
|-------|-------|
| Extension Type | `microsoft.extensions.k8sgpt` |
| Supported Clusters | `managedClusters` (AKS), `connectedClusters` (Arc) |
| Scope | `cluster` |
| Release Train | `dev` |
| Default Namespace | `k8sgpt-system` |
| Helm Chart | `k8sgptextdemo.azurecr.io/helm/k8sgpt-azure-extensions:0.1.0` |

## Configuration Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ai.backend` | string | `azureopenai` | AI provider (azureopenai, openai, ollama) |
| `ai.model` | string | `gpt-4o` | Model name |
| `ai.baseUrl` | string | — | AI endpoint URL |
| `ai.engine` | string | — | Deployment name (Azure OpenAI) |
| `ai.temperature` | string | `0.7` | Sampling temperature |
| `analyze.filters` | string | `AzureExtensionConfig,AzureArcAgents` | Analyzers to enable |

## Protected Settings

| Key | Description |
|-----|-------------|
| `ai_secret.aiPassword` | AI provider API key (stored as K8s Secret) |

## Install Command

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name <cluster> \
  --resource-group <rg> \
  --cluster-type managedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=azureopenai \
    ai.model=gpt-4o \
    ai.baseUrl=https://<resource>.openai.azure.com \
    ai.engine=<deployment-name> \
  --configuration-protected-settings \
    ai_secret.aiPassword=<api-key>
```

## Container Images

| Image | Purpose |
|-------|---------|
| `k8sgptextdemo.azurecr.io/k8sgpt-azure-extensions:latest` | k8sgpt server with Azure Extensions analyzer |
| `k8sgptextdemo.azurecr.io/k8sgpt-dashboard:latest` | Streamlit dashboard UI |
