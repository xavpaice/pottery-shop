{{- define "clay.supportbundle" -}}
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: clay-support-bundle
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}

    # Clay application logs
    - logs:
        selector:
          - app.kubernetes.io/name=clay
        namespace: {{ .Release.Namespace }}
        limits:
          maxAge: 720h
          maxLines: 10000
        name: clay/app-logs

    # PostgreSQL logs (CNPG managed)
    {{- if .Values.postgres.managed }}
    - logs:
        selector:
          - cnpg.io/cluster={{ include "clay.fullname" . }}-postgres
        namespace: {{ .Release.Namespace }}
        limits:
          maxAge: 720h
          maxLines: 10000
        name: clay/postgres-logs
    {{- end }}

    # CNPG operator logs
    - logs:
        selector:
          - app.kubernetes.io/name=cloudnative-pg
        namespace: {{ .Release.Namespace }}
        limits:
          maxAge: 720h
          maxLines: 5000
        name: clay/cnpg-operator-logs

    # cert-manager logs
    - logs:
        selector:
          - app.kubernetes.io/name=cert-manager
        namespace: {{ .Release.Namespace }}
        limits:
          maxAge: 168h
          maxLines: 5000
        name: clay/cert-manager-logs

    # Replicated SDK logs
    - logs:
        selector:
          - app.kubernetes.io/name=clay-sdk
        namespace: {{ .Release.Namespace }}
        limits:
          maxAge: 168h
          maxLines: 5000
        name: clay/sdk-logs

    # App readiness check via in-cluster service
    - http:
        name: clay-readiness
        get:
          url: http://{{ include "clay.fullname" . }}.{{ .Release.Namespace }}.svc.cluster.local:{{ .Values.service.port }}/readyz
        timeout: 10s

    # App health check via in-cluster service
    - http:
        name: clay-health
        get:
          url: http://{{ include "clay.fullname" . }}.{{ .Release.Namespace }}.svc.cluster.local:{{ .Values.service.port }}/healthz
        timeout: 10s

    {{- if .Values.postgres.managed }}
    # PostgreSQL connectivity check from app pod
    - exec:
        name: postgres-connectivity
        collectorName: postgres-connectivity
        selector:
          - app.kubernetes.io/name=clay
        namespace: {{ .Release.Namespace }}
        command: ["sh"]
        args: ["-c", "pg_isready -h {{ include "clay.fullname" . }}-postgres-rw -p 5432 2>&1"]
        timeout: 10s
    {{- end }}

    # Storage class info
    - storageClasses: {}

    # PVC info
    - pvcs:
        namespace: {{ .Release.Namespace }}

    # Service and endpoint info
    - services:
        namespace: {{ .Release.Namespace }}

  analyzers:
    # --- App readiness HTTP check ---
    - textAnalyze:
        checkName: Clay application readiness
        fileName: clay-readiness/result.json
        regex: '"status":"ready"'
        outcomes:
          - fail:
              when: "false"
              message: |
                The Clay application is not ready. The /readyz endpoint did not
                return "ready", which typically means the database is unreachable.
                Check the clay/app-logs and clay/postgres-logs in this bundle for
                connection errors.
          - pass:
              when: "true"
              message: Clay application is ready and healthy

    # --- App health HTTP check ---
    - textAnalyze:
        checkName: Clay application health
        fileName: clay-health/result.json
        regex: '"status":"ok"'
        outcomes:
          - fail:
              when: "false"
              message: |
                The Clay application is not healthy. The /healthz endpoint did not
                return "ok", which means the application process is down or
                unresponsive. Check clay/app-logs for crash details and pod events
                for OOMKilled or other termination reasons.
          - pass:
              when: "true"
              message: Clay application is healthy

    # --- Deployment status analyzers ---
    - deploymentStatus:
        checkName: Clay application deployment
        name: {{ include "clay.fullname" . }}
        namespace: {{ .Release.Namespace }}
        outcomes:
          - fail:
              when: "< 1"
              message: |
                The Clay application deployment has no ready replicas. The pottery
                shop is down and customers cannot browse or place orders. Check pod
                events and clay/app-logs for crash loops or image pull errors.
          - warn:
              when: "< {{ .Values.replicaCount }}"
              message: |
                The Clay application has fewer ready replicas than configured
                (expected {{ .Values.replicaCount }}). The shop may be degraded.
          - pass:
              message: Clay application deployment is healthy

    {{- if (index .Values "cloudnative-pg" "enabled") }}
    - deploymentStatus:
        checkName: CNPG operator deployment
        name: {{ include "clay.fullname" . }}-cloudnative-pg
        namespace: {{ .Release.Namespace }}
        outcomes:
          - fail:
              when: "< 1"
              message: |
                The CloudNativePG operator has no ready replicas. PostgreSQL cluster
                management is unavailable -- new clusters will not be created and
                existing clusters will not be monitored for failover. Check
                clay/cnpg-operator-logs for errors.
          - pass:
              message: CNPG operator is healthy
    {{- end }}

    {{- if (index .Values "cert-manager" "enabled") }}
    - deploymentStatus:
        checkName: cert-manager deployment
        name: {{ include "clay.fullname" . }}-cert-manager
        namespace: {{ .Release.Namespace }}
        outcomes:
          - fail:
              when: "< 1"
              message: |
                The cert-manager controller has no ready replicas. TLS certificate
                issuance and renewal will fail, which may cause ingress to serve
                expired certificates. Check clay/cert-manager-logs for errors.
          - pass:
              message: cert-manager is healthy
    {{- end }}

    # --- Log analysis: database connection failures ---
    - textAnalyze:
        checkName: Database connection errors in app logs
        fileName: clay/app-logs/*/clay.log
        regex: "DB ping failed|Failed to create connection pool|connection refused"
        outcomes:
          - fail:
              when: "true"
              message: |
                Database connection errors found in the Clay application logs.
                The app cannot reach PostgreSQL, which causes the readiness probe
                to fail and prevents the shop from serving requests.
                Check that the CNPG Cluster is healthy (kubectl get clusters.postgresql.cnpg.io -n {{ .Release.Namespace }})
                and that the postgres-rw service is reachable.
          - pass:
              when: "false"
              message: No database connection errors found in app logs

    # --- Log analysis: migration failures ---
    - textAnalyze:
        checkName: Database migration errors
        fileName: clay/app-logs/*/clay.log
        regex: "Failed to run migrations|goose.*error"
        outcomes:
          - fail:
              when: "true"
              message: |
                Database migration errors found in the Clay application logs.
                The app failed to apply schema changes on startup, which may cause
                missing tables or columns. Check the full log output for the specific
                migration that failed and verify the database is accessible.
                See https://github.com/pressly/goose for migration troubleshooting.
          - pass:
              when: "false"
              message: No migration errors found in app logs

    # --- Storage class check ---
    - storageClass:
        checkName: Default storage class
        storageClassName: ""
        outcomes:
          - fail:
              message: |
                No default storage class found. Clay requires a default storage
                class for PostgreSQL data persistence and uploaded product images.
                Create a default storage class or set persistence.storageClass and
                postgres.cluster.storage.storageClass explicitly in your values.
                See https://kubernetes.io/docs/concepts/storage/storage-classes/
          - pass:
              message: Default storage class is available

    # --- Node readiness check ---
    - nodeResources:
        checkName: All nodes ready
        filters:
          cpuCapacity: "0"
        outcomes:
          - fail:
              when: "count(unready) > 0"
              message: |
                One or more worker nodes are not in Ready state. Pods may be
                unschedulable or evicted, causing application downtime. Run
                'kubectl get nodes' and 'kubectl describe node <name>' to diagnose
                the unhealthy node(s). Common causes: disk pressure, memory pressure,
                network issues, or kubelet not running.
          - pass:
              message: All nodes are in Ready state
{{- end -}}
