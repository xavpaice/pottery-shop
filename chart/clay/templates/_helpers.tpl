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
{{- if not (.Values.secrets.ADMIN_PASS | trim) }}
  {{- fail "secrets.ADMIN_PASS must be set (value must not be blank)" }}
{{- end }}
{{- if not (.Values.secrets.SESSION_SECRET | trim) }}
  {{- fail "secrets.SESSION_SECRET must be set (value must not be blank)" }}
{{- end }}
{{- end }}

{{/*
Validate required ingress values -- fail fast at render time.
Called from ingress.yaml inside the {{- if .Values.ingress.enabled }} guard.
*/}}
{{- define "clay.validateIngress" -}}
{{- if not (.Values.ingress.host | trim) }}
  {{- fail "ingress.host must be set (value must not be blank)" }}
{{- end }}
{{- if not (.Values.ingress.tls.mode | trim) }}
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

{{/*
Validate database configuration — fail fast at render time if no database source is configured.
Called from deployment.yaml. Fails if neither postgres.managed is true nor postgres.external.dsn is set.
This guards against the "no database at all" misconfiguration where postgres.managed=false but no DSN is provided. (D-03)
*/}}
{{- define "clay.validateDB" -}}
{{- if and (not .Values.postgres.managed) (not .Values.postgres.external.dsn) }}
  {{- fail "postgres.managed or postgres.external.dsn required" }}
{{- end }}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "replicated.imagePullSecrets" -}}
  {{- $pullSecrets := list }}

  {{- with ((.Values.global).imagePullSecrets) -}}
    {{- range . -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end }}
    {{- end -}}
  {{- end -}}

  {{/* use image pull secrets provided as values */}}
  {{- with .Values.images -}}
    {{- range .pullSecrets -}}
      {{- if kindIs "map" . -}}
        {{- $pullSecrets = append $pullSecrets .name -}}
      {{- else -}}
        {{- $pullSecrets = append $pullSecrets . -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}

  {{/* use the pull secret created by the SDK */}}
  {{- if hasKey ((.Values.global).replicated) "dockerconfigjson" }}
    {{- $pullSecrets = append $pullSecrets "enterprise-pull-secret" -}}
  {{- end -}}


  {{- if (not (empty $pullSecrets)) -}}
imagePullSecrets:
    {{- range $pullSecrets | uniq }}
  - name: {{ . }}
    {{- end }}
  {{- end }}
{{- end -}}