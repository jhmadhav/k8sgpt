# Extension Registration: microsoft.extensions.k8sgpt

## Overview

AI-powered diagnostics for Azure Kubernetes Extensions using k8sgpt. Detects, diagnoses, and explains extension failures on AKS and Arc-enabled clusters by tracing the full chain: ExtensionConfig CR → ExtensionEvents → Helm Release → Deployments → Pods → Logs.

## Extension Details

| Field | Value |
|-------|-------|
| Extension Type | `microsoft.extensions.k8sgpt` |
| Supported Clusters | `managedClusters` (AKS), `connectedClusters` (Arc) |
| Scope | `cluster` |
| Release Train | `dev` |
| Default Namespace | `k8sgpt-system` |
| Helm Chart | `k8sgptextdemo.azurecr.io/helm/k8sgpt-azure-extensions:0.1.0` |

## Registration Artifacts

| File | Purpose |
|------|---------|
| `type-registration.json` | Extension type definition — supported clusters, scope, billing, ICM routing |
| `package-config.json` | Package configuration — config schema, reconciler settings, supported cluster types |
| `version-config.json` | Version configuration — Helm chart ACR path, release train, regions |

---

## Supported Configuration Settings

These are passed via `--configuration-settings` when installing the extension:

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --configuration-settings key=value key2=value2
```

### AI Backend Settings

| Setting | Type | Default | Required | Description |
|---------|------|---------|----------|-------------|
| `ai.backend` | string | `azureopenai` | Yes | AI provider backend. Supported values: `azureopenai`, `openai`, `ollama`, `localai`, `noopai` |
| `ai.model` | string | `gpt-4o` | Yes | AI model name. For Azure OpenAI, this is the model name (e.g., `gpt-4o`, `gpt-4o-mini`). For OpenAI/GitHub Models, use the model ID |
| `ai.baseUrl` | string | — | Yes (for azureopenai) | AI provider endpoint URL. For Azure OpenAI: `https://<resource>.openai.azure.com`. For GitHub Models: `https://models.github.ai/inference`. For Ollama: `http://ollama:11434` |
| `ai.engine` | string | — | Yes (for azureopenai) | Deployment name in Azure OpenAI. Must match the deployment created in Azure AI Foundry |
| `ai.temperature` | string | `0.7` | No | AI sampling temperature (0.0-1.0). Lower = more deterministic, higher = more creative |

### Analyzer Settings

| Setting | Type | Default | Required | Description |
|---------|------|---------|----------|-------------|
| `analyze.filters` | string | `AzureExtensionConfig,AzureArcAgents` | No | Comma-separated list of analyzers to enable. Available analyzers: `AzureExtensionConfig` (extension CRD diagnostics), `AzureArcAgents` (Arc agent health) |

### Image Settings

| Setting | Type | Default | Required | Description |
|---------|------|---------|----------|-------------|
| `image.repository` | string | `k8sgptextdemo.azurecr.io/k8sgpt-azure-extensions` | No | k8sgpt server container image repository |
| `image.tag` | string | `latest` | No | k8sgpt server container image tag |
| `image.pullPolicy` | string | `IfNotPresent` | No | Image pull policy (`Always`, `IfNotPresent`, `Never`) |
| `dashboard.image.repository` | string | `k8sgptextdemo.azurecr.io/k8sgpt-dashboard` | No | Dashboard UI container image repository |
| `dashboard.image.tag` | string | `latest` | No | Dashboard UI container image tag |

### Infrastructure Settings

| Setting | Type | Default | Required | Description |
|---------|------|---------|----------|-------------|
| `replicaCount` | int | `1` | No | Number of k8sgpt server replicas |
| `serviceAccount.create` | bool | `true` | No | Create a dedicated ServiceAccount |
| `serviceAccount.name` | string | `k8sgpt-azure-extensions` | No | ServiceAccount name |
| `rbac.create` | bool | `true` | No | Create RBAC ClusterRole and ClusterRoleBinding |

---

## Supported Protected Settings

These are passed via `--configuration-protected-settings` and stored as Kubernetes Secrets:

```bash
az k8s-extension create \
  --configuration-protected-settings key=value
```

| Setting | Type | Required | Description |
|---------|------|----------|-------------|
| `ai_secret.aiPassword` | string | Yes | AI provider API key. For Azure OpenAI: the API key from Azure portal. For GitHub Models: a GitHub PAT with `models:read` scope. Stored as a K8s Secret, never in ConfigMaps or logs |

### Using an Existing Secret

Instead of passing the key via protected settings, you can reference a pre-created K8s Secret:

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `ai_secret.existingSecretName` | string | — | Name of an existing K8s Secret containing the AI key (takes precedence over `aiPassword`) |
| `ai_secret.existingSecretKey` | string | `api-key` | Key within the existing Secret that holds the AI key |

---

## Install Examples

### Azure OpenAI

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name my-aks-cluster \
  --resource-group my-rg \
  --cluster-type managedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=azureopenai \
    ai.model=gpt-4o \
    ai.baseUrl=https://my-openai.openai.azure.com \
    ai.engine=gpt-4o \
  --configuration-protected-settings \
    ai_secret.aiPassword=<azure-openai-key>
```

### GitHub Models (Free)

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name my-aks-cluster \
  --resource-group my-rg \
  --cluster-type managedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=openai \
    ai.model=gpt-4o \
    ai.baseUrl=https://models.github.ai/inference \
  --configuration-protected-settings \
    ai_secret.aiPassword=<github-pat>
```

### Ollama (Local Inference)

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name my-aks-cluster \
  --resource-group my-rg \
  --cluster-type managedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=ollama \
    ai.model=llama3.1 \
    ai.baseUrl=http://ollama.ollama-system.svc:11434 \
  --configuration-protected-settings \
    ai_secret.aiPassword=not-needed
```

### Without AI (Diagnostics Only)

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name my-aks-cluster \
  --resource-group my-rg \
  --cluster-type managedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=noopai \
    ai.model=noopai \
  --configuration-protected-settings \
    ai_secret.aiPassword=unused
```

### Arc-enabled Cluster

```bash
az k8s-extension create \
  --extension-type microsoft.extensions.k8sgpt \
  --name k8sgpt \
  --cluster-name my-arc-cluster \
  --resource-group my-rg \
  --cluster-type connectedClusters \
  --release-train dev \
  --configuration-settings \
    ai.backend=azureopenai \
    ai.model=gpt-4o \
    ai.baseUrl=https://my-openai.openai.azure.com \
    ai.engine=gpt-4o \
  --configuration-protected-settings \
    ai_secret.aiPassword=<azure-openai-key>
```

---

## What the Analyzers Detect

### AzureExtensionConfig Analyzer

Scans `extensionconfigs` CRDs (both `aks.clusterconfig.azure.com` and `clusterconfig.azure.com`) across all namespaces:

| Check | What it catches |
|-------|----------------|
| `status.status == Failed` | Extension install/upgrade failures |
| `status.reconciliationError` | Helm release errors, timeouts, manifest parse failures |
| `status.extensionState` | Non-healthy states (Error, Updating, Unavailable) |
| `status.helmReleaseStatus` | Failed/pending Helm releases |
| `status.syncStatus.isSyncedWithAzure == false` | ARM sync drift (48h disconnection) |
| `lastSuccessfulReconciledTime == ""` | Extensions that never successfully installed |
| Helm release secrets | Failed upgrades, rollback state |
| Deployments | Unavailable replicas, not Available/Progressing |
| Failed Jobs | Helm post-install/upgrade hook failures + pod logs |
| Pod diagnostics | CrashLoopBackOff, ImagePullBackOff, OOMKilled, high restart counts |
| Pod logs | Tails last 50 lines for error/fatal/panic patterns |
| K8s Warning events | Image pull errors, scheduling failures, OOM events |

### AzureArcAgents Analyzer

Only runs on Arc-enabled clusters (when `azure-arc` namespace exists):

| Check | What it catches |
|-------|----------------|
| Arc agent deployments | extension-manager, config-agent, etc. — unavailable replicas |
| Arc agent pods | CrashLoopBackOff, ImagePullBackOff in azure-arc namespace |
| Required secrets | Missing `azure-identity-certificate`, `kube-aad-proxy-certificate` |
| AzureClusterIdentityRequest | Missing token references, broken MSI token delivery |
| ConfigSyncStatus | ARM sync health, stale sync times |

---

## Container Images

| Image | Purpose |
|-------|---------|
| `k8sgptextdemo.azurecr.io/k8sgpt-azure-extensions:latest` | k8sgpt server with Azure Extensions analyzer integration |
| `k8sgptextdemo.azurecr.io/k8sgpt-dashboard:latest` | Streamlit dashboard UI ("Azure Extensions Health Copilot") |

## RBAC Permissions

The extension creates a ClusterRole with these permissions:

| API Group | Resources | Verbs |
|-----------|-----------|-------|
| `aks.clusterconfig.azure.com` | extensionconfigs, configsyncstatuses | get, list, watch |
| `clusterconfig.azure.com` | extensionconfigs, extensionevents, configsyncstatuses, healthstates, arccertificates, azureclusteridentityrequests, azureextensionidentities, agentconfigs | get, list, watch |
| `""` (core) | namespaces, pods, pods/log, services, endpoints, events, configmaps, persistentvolumeclaims, nodes, secrets | get, list, watch |
| `apps` | deployments, daemonsets, replicasets, statefulsets | get, list, watch |
| `batch` | jobs, cronjobs | get, list, watch |
| `networking.k8s.io` | ingresses | get, list, watch |
| `admissionregistration.k8s.io` | validatingwebhookconfigurations, mutatingwebhookconfigurations | get, list, watch |
