{{/*
Expand the name of the chart.
*/}}
{{- define "synthetic-checker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "synthetic-checker.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "synthetic-checker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "synthetic-checker.labels" -}}
{{ include "synthetic-checker.extraLabels" . }}
{{ include "synthetic-checker.selectorLabels" . }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "synthetic-checker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "synthetic-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Extra labels
*/}}
{{- define "synthetic-checker.extraLabels" -}}
helm.sh/chart: {{ include "synthetic-checker.chart" . }}
helm.sh/release-name: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if or .Chart.AppVersion .Values.image.tag }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
{{- end }}
{{- end }}

{{/*
Prometheus labels
*/}}
{{- define "synthetic-checker.prometheusLabels" -}}
{{- if .Values.prometheus.enabled }}
prometheus.io/scrape: "true"
prometheus.io/endpoint: {{ .Values.prometheus.endpoint | quote }}
prometheus.io/port: {{ .Values.prometheus.port | quote }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "synthetic-checker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "synthetic-checker.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
