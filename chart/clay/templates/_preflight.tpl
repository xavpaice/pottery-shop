{{- define "clay.preflight" -}}
apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: clay-preflight-checks
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
    {{- if and (not .Values.postgres.managed) .Values.postgres.external.dsn }}
    - runDaemonSet:
        name: external-db-check
        namespace: {{ .Release.Namespace }}
        timeout: 30s
        podSpec:
          containers:
            - name: check
              image: {{ .Values.waitForPostgres.image.registry }}/{{ .Values.waitForPostgres.image.repository }}:{{ .Values.waitForPostgres.image.tag }}
              command: ["sh", "-c", "pg_isready -h $(echo $DSN | sed 's|.*@||;s|/.*||;s|:.*||') -t 5 2>&1 || echo CONNECTION_FAILED"]
              env:
                - name: DSN
                  value: {{ .Values.postgres.external.dsn | quote }}
          restartPolicy: Always
    {{- end }}
    {{- if .Values.config.SMTP_HOST }}
    - runDaemonSet:
        name: smtp-check
        namespace: {{ .Release.Namespace }}
        timeout: 30s
        podSpec:
          containers:
            - name: check
              image: busybox:1.36
              command: ["sh", "-c", "nc -z -w5 {{ .Values.config.SMTP_HOST }} {{ .Values.config.SMTP_PORT | default "587" }} && echo SMTP_OK || echo SMTP_FAILED"]
          restartPolicy: Always
    {{- end }}

  analyzers:
    {{- if and (not .Values.postgres.managed) .Values.postgres.external.dsn }}
    - docString: |
        Title: External Database Connectivity
        Requirement:
          - PostgreSQL must be reachable at the configured DSN
        Verifies the external PostgreSQL database accepts connections from cluster nodes.
      textAnalyze:
        checkName: External database connectivity
        fileName: external-db-check/*.log
        regex: "accepting connections"
        outcomes:
          - fail:
              when: "false"
              message: |
                Cannot connect to the external PostgreSQL database.
                Verify that postgres.external.dsn is correct and the database
                is reachable from this cluster. Check network policies, firewall
                rules, and that the database server is running.
          - pass:
              when: "true"
              message: External PostgreSQL database is reachable
    {{- end }}

    {{- if .Values.config.SMTP_HOST }}
    - docString: |
        Title: SMTP Server Connectivity
        Requirement:
          - SMTP server must be reachable at {{ .Values.config.SMTP_HOST }}:{{ .Values.config.SMTP_PORT | default "587" }}
        Verifies the mail server is reachable for order notification emails.
      textAnalyze:
        checkName: SMTP server connectivity
        fileName: smtp-check/*.log
        regex: "SMTP_OK"
        outcomes:
          - fail:
              when: "false"
              message: |
                Cannot connect to the SMTP server at {{ .Values.config.SMTP_HOST }}:{{ .Values.config.SMTP_PORT | default "587" }}.
                Order notification emails will not be sent. Verify SMTP_HOST and
                SMTP_PORT are correct, and that the SMTP server is reachable from
                this cluster. Check firewall rules and network policies.
          - pass:
              when: "true"
              message: SMTP server is reachable
    {{- end }}
    - docString: |
        Title: Minimum CPU Available
        Requirement:
          - Minimum: 2 CPU cores allocatable
          - Recommended: 4 CPU cores allocatable
        Ensures the cluster has enough CPU for the application, PostgreSQL, and operators.
      nodeResources:
        checkName: Minimum CPU available
        outcomes:
          - fail:
              when: "sum(cpuAllocatable) < 2"
              message: |
                The cluster has fewer than 2 allocatable CPU cores. Clay requires
                at least 2 CPU cores to run the application, PostgreSQL, and
                operators. Add more nodes or increase node size.
          - warn:
              when: "sum(cpuAllocatable) < 4"
              message: |
                The cluster has fewer than 4 allocatable CPU cores. At least 4
                cores are recommended for production workloads.
          - pass:
              message: Sufficient CPU resources available

    - docString: |
        Title: Minimum Memory Available
        Requirement:
          - Minimum: 2Gi memory allocatable
          - Recommended: 4Gi memory allocatable
        Ensures the cluster has enough memory for the application, PostgreSQL, and operators.
      nodeResources:
        checkName: Minimum memory available
        outcomes:
          - fail:
              when: "sum(memoryAllocatable) < 2Gi"
              message: |
                The cluster has less than 2Gi of allocatable memory. Clay requires
                at least 2Gi to run the application, PostgreSQL, and operators.
                Add more nodes or increase node size.
          - warn:
              when: "sum(memoryAllocatable) < 4Gi"
              message: |
                The cluster has less than 4Gi of allocatable memory. At least 4Gi
                is recommended for production workloads.
          - pass:
              message: Sufficient memory available

    - docString: |
        Title: Kubernetes Version
        Requirement:
          - Minimum: 1.27.0
          - Recommended: 1.29.0
        Ensures the cluster meets the minimum Kubernetes API version requirements.
        Links:
          - https://kubernetes.io/releases/
      clusterVersion:
        checkName: Kubernetes version
        outcomes:
          - fail:
              when: "< 1.27.0"
              message: |
                Kubernetes version is below the minimum supported version (1.27.0).
                Upgrade your cluster to Kubernetes 1.27 or later.
                See https://kubernetes.io/releases/ for supported versions.
          - warn:
              when: "< 1.29.0"
              message: |
                Kubernetes 1.29.0 or later is recommended. Your version is
                supported but nearing end of life.
          - pass:
              when: ">= 1.29.0"
              message: Kubernetes version is supported

    - docString: |
        Title: Default Storage Class
        Requirement:
          - A default StorageClass must be available
        Ensures persistent volumes can be provisioned for PostgreSQL and uploads.
        Links:
          - https://kubernetes.io/docs/concepts/storage/storage-classes/
      storageClass:
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

    - docString: |
        Title: Kubernetes Distribution
        Requirement:
          - Must not be docker-desktop or microk8s
        Ensures the cluster is running a supported Kubernetes distribution.
        Links:
          - https://clay.nz/docs/supported-platforms
      distribution:
        checkName: Kubernetes distribution
        outcomes:
          - fail:
              when: "== docker-desktop"
              message: |
                Docker Desktop is not a supported Kubernetes distribution for Clay.
                Use a production-grade distribution such as k3s, EKS, GKE, or AKS.
                See https://clay.nz/docs/supported-platforms for details.
          - fail:
              when: "== microk8s"
              message: |
                MicroK8s is not a supported Kubernetes distribution for Clay.
                Use a production-grade distribution such as k3s, EKS, GKE, or AKS.
                See https://clay.nz/docs/supported-platforms for details.
          - warn:
              when: "== minikube"
              message: |
                Minikube detected. This is suitable for development and testing
                only. Use a production-grade distribution for production deployments.
          - pass:
              message: Kubernetes distribution is supported
{{- end -}}
