{{- define "k8sgpt-azure-extensions.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "k8sgpt-azure-extensions.labels" -}}
app.kubernetes.io/name: k8sgpt-azure-extensions
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "k8sgpt-azure-extensions.selectorLabels" -}}
app.kubernetes.io/name: k8sgpt-azure-extensions
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
