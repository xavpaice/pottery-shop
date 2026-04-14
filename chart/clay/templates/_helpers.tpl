{{/*
Expand the name of the chart.
*/}}
{{- define "clay.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "clay.fullname" -}}
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
Common labels
*/}}
{{- define "clay.labels" -}}
helm.sh/chart: {{ include "clay.chart" . }}
{{ include "clay.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "clay.selectorLabels" -}}
app.kubernetes.io/name: {{ include "clay.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Chart label
*/}}
{{- define "clay.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Validate required secrets — fail fast at render time rather than at runtime.
*/}}
{{- define "clay.validateSecrets" -}}
{{- if not .Values.secrets.ADMIN_PASS }}
  {{- fail "secrets.ADMIN_PASS must be set" }}
{{- end }}
{{- if not .Values.secrets.SESSION_SECRET }}
  {{- fail "secrets.SESSION_SECRET must be set" }}
{{- end }}
{{- end }}

{{/*
Validate required ingress values -- fail fast at render time.
Called from ingress.yaml inside the {{- if .Values.ingress.enabled }} guard.
*/}}
{{- define "clay.validateIngress" -}}
{{- if not .Values.ingress.host }}
  {{- fail "ingress.host must be set" }}
{{- end }}
{{- if not .Values.ingress.tls.mode }}
  {{- fail "ingress.tls.mode must be set (letsencrypt|selfsigned|custom)" }}
{{- end }}
{{- if and (ne .Values.ingress.tls.mode "letsencrypt") (ne .Values.ingress.tls.mode "selfsigned") (ne .Values.ingress.tls.mode "custom") }}
  {{- fail "ingress.tls.mode must be one of: letsencrypt, selfsigned, custom" }}
{{- end }}
{{- if eq .Values.ingress.tls.mode "letsencrypt" }}
  {{- if not .Values.ingress.tls.acme.email }}
    {{- fail "ingress.tls.acme.email required for letsencrypt mode" }}
  {{- end }}
{{- end }}
{{- if eq .Values.ingress.tls.mode "custom" }}
  {{- if not .Values.ingress.tls.secretName }}
    {{- fail "ingress.tls.secretName required for custom mode" }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Return the TLS secret name for the Ingress resource.
Custom mode: user-provided secretName from values.
Letsencrypt/selfsigned: derived from release fullname.
Used by both ingress.yaml (this phase) and Certificate CR (Phase 4).
*/}}
{{- define "clay.tlsSecretName" -}}
{{- if eq .Values.ingress.tls.mode "custom" -}}
  {{- .Values.ingress.tls.secretName -}}
{{- else -}}
  {{- printf "%s-tls" (include "clay.fullname" .) -}}
{{- end -}}
{{- end }}
