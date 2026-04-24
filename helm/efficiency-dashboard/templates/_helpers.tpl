{{/*
Expand the name of the chart.
*/}}
{{- define "efficiency-dashboard.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "efficiency-dashboard.fullname" -}}
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
{{- define "efficiency-dashboard.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "efficiency-dashboard.labels" -}}
helm.sh/chart: {{ include "efficiency-dashboard.chart" . }}
{{ include "efficiency-dashboard.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "efficiency-dashboard.selectorLabels" -}}
app.kubernetes.io/name: {{ include "efficiency-dashboard.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "efficiency-dashboard.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "efficiency-dashboard.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Elasticsearch labels
*/}}
{{- define "efficiency-dashboard.elasticsearch.labels" -}}
helm.sh/chart: {{ include "efficiency-dashboard.chart" . }}
{{ include "efficiency-dashboard.elasticsearch.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "efficiency-dashboard.elasticsearch.selectorLabels" -}}
app.kubernetes.io/name: {{ include "efficiency-dashboard.name" . }}-elasticsearch
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
PostgreSQL labels
*/}}
{{- define "efficiency-dashboard.postgresql.labels" -}}
helm.sh/chart: {{ include "efficiency-dashboard.chart" . }}
{{ include "efficiency-dashboard.postgresql.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "efficiency-dashboard.postgresql.selectorLabels" -}}
app.kubernetes.io/name: {{ include "efficiency-dashboard.name" . }}-postgresql
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Server labels
*/}}
{{- define "efficiency-dashboard.server.labels" -}}
helm.sh/chart: {{ include "efficiency-dashboard.chart" . }}
{{ include "efficiency-dashboard.server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "efficiency-dashboard.server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "efficiency-dashboard.name" . }}-server
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Portal labels
*/}}
{{- define "efficiency-dashboard.portal.labels" -}}
helm.sh/chart: {{ include "efficiency-dashboard.chart" . }}
{{ include "efficiency-dashboard.portal.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "efficiency-dashboard.portal.selectorLabels" -}}
app.kubernetes.io/name: {{ include "efficiency-dashboard.name" . }}-portal
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
