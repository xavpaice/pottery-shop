.PHONY: build test test-verbose clean run run-local run-stop docker helm-lint lint integration-test cmx-test cmx-test-teardown ec-test ec-test-teardown

BINARY := pottery-server
GO := go
K3D_CLUSTER := pottery
K3D_IMAGE := pottery-shop:dev

## build: compile the server binary
build:
	CGO_ENABLED=0 $(GO) build -o $(BINARY) ./cmd/server

## test: run all tests
test:
	CGO_ENABLED=0 TESTCONTAINERS_RYUK_DISABLED=true $(GO) test ./...

## test-verbose: run all tests with verbose output
test-verbose:
	CGO_ENABLED=0 TESTCONTAINERS_RYUK_DISABLED=true $(GO) test -v ./...

## test-coverage: run tests with coverage report
test-coverage:
	CGO_ENABLED=0 TESTCONTAINERS_RYUK_DISABLED=true $(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html

## run: build image, deploy to local k3d cluster, port-forward to localhost:8080
run:
	@command -v k3d >/dev/null 2>&1 || { echo "Error: k3d not installed. Run: brew install k3d"; exit 1; }
	@command -v helm >/dev/null 2>&1 || { echo "Error: helm not installed. Run: brew install helm"; exit 1; }
	@echo "--- Ensuring k3d cluster '$(K3D_CLUSTER)' exists ---"
	@k3d cluster list $(K3D_CLUSTER) >/dev/null 2>&1 || \
		k3d cluster create $(K3D_CLUSTER) --no-lb --wait
	@echo "--- Building image ---"
	docker build -t $(K3D_IMAGE) .
	@echo "--- Importing image into k3d ---"
	k3d image import $(K3D_IMAGE) -c $(K3D_CLUSTER)
	@echo "--- Updating chart dependencies ---"
	helm dependency update chart/clay/
	@echo "--- Installing chart ---"
	@kubectl --context k3d-$(K3D_CLUSTER) create namespace clay 2>/dev/null || true
	helm upgrade --install clay chart/clay/ \
		--namespace clay \
		--set image.repository=pottery-shop \
		--set image.tag=dev \
		--set image.pullPolicy=Never \
		--set ingress.enabled=false \
		--set persistence.enabled=false \
		--set secrets.ADMIN_PASS=localdev \
		--set secrets.SESSION_SECRET=localdev-session-secret-min-32chars \
		--timeout 5m
	@echo "--- Waiting for Postgres cluster to become ready ---"
	@for i in $$(seq 1 60); do \
		if kubectl --context k3d-$(K3D_CLUSTER) get clusters.postgresql.cnpg.io -n clay clay-postgres -o jsonpath='{.status.phase}' 2>/dev/null | grep -q "Cluster in healthy state"; then \
			echo "Postgres ready"; break; \
		fi; \
		printf "."; sleep 5; \
	done
	@echo ""
	@echo "--- Waiting for app deployment ---"
	@kubectl --context k3d-$(K3D_CLUSTER) rollout status deployment/clay -n clay --timeout=120s
	@echo "--- Cluster ready ---"
	@kubectl --context k3d-$(K3D_CLUSTER) get pods -n clay
	@echo ""
	@echo "Port-forward: kubectl --context k3d-$(K3D_CLUSTER) port-forward -n clay svc/clay 8080:80"
	@echo "Admin login:  admin / localdev"
	@echo "Stop cluster: make run-stop"

## run-local: build and run the server binary directly (no k8s)
run-local: build
	./$(BINARY)

## run-stop: delete the local k3d cluster
run-stop:
	k3d cluster delete $(K3D_CLUSTER)

## tidy: tidy and verify module dependencies
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## helm-lint: lint the Helm chart
helm-lint:
	helm lint chart/clay/ -f chart/clay/ci/ci-bundled-values.yaml
	helm lint chart/clay/ -f chart/clay/ci/ci-external-db-values.yaml

## helm-test: run helm template behavioral tests (Phase 3 validation)
helm-test:
	chart/tests/helm-template-test.sh

## lint: run all linters
lint: helm-lint helm-test

## docker: build Docker image
docker:
	docker build -t ghcr.io/xavpaice/pottery-shop:latest .

## deploy: deploy via Helm (raw manifests not included; use the Helm chart)
deploy:
	@echo "Raw Kubernetes manifests are not included. Use 'helm upgrade --install clay ./chart/clay -n clay' instead."
	@exit 1

GHCR_USERNAME ?= xavpaice
IMAGE_REPO := ghcr.io/xavpaice/pottery-shop
IMAGE_TAG := test-$(shell git rev-parse --short HEAD)
CLUSTER_NAME := pottery-integration-$(shell date +%s)

## integration-test: build image, create CMX cluster, install chart, verify, teardown
integration-test:
	@for tool in replicated docker helm kubectl jq; do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			if [ "$$tool" = "replicated" ]; then \
				echo "Error: replicated CLI not installed. See https://docs.replicated.com/reference/replicated-cli-installing"; \
			else \
				echo "Error: $$tool CLI not installed"; \
			fi; \
			exit 1; \
		fi; \
	done
	@test -n "$$REPLICATED_API_TOKEN" || { echo "Error: REPLICATED_API_TOKEN not set"; exit 1; }
	@test -n "$$GHCR_TOKEN" || { echo "Error: GHCR_TOKEN not set"; exit 1; }
	@CLUSTER_NAME=$(CLUSTER_NAME) \
	IMAGE_REPO=$(IMAGE_REPO) \
	IMAGE_TAG=$(IMAGE_TAG) \
	GHCR_USERNAME=$(GHCR_USERNAME) \
	GHCR_TOKEN=$$GHCR_TOKEN \
	bash -ec ' \
		cleanup() { \
			EXIT_CODE=$$?; \
			if [ $$EXIT_CODE -ne 0 ] && [ -n "$$KUBECONFIG" ] && [ -f "$$KUBECONFIG" ]; then \
				echo "--- Debug info ---"; \
				kubectl get pods -n clay -o wide 2>/dev/null || true; \
				kubectl get events -n clay --sort-by=.lastTimestamp 2>/dev/null || true; \
				kubectl describe pods -n clay 2>/dev/null || true; \
				kubectl logs -n clay -l app.kubernetes.io/name=clay --tail=50 2>/dev/null || true; \
			fi; \
			echo "--- Cleaning up ---"; \
			if [ -n "$$CLUSTER_ID" ]; then replicated cluster rm $$CLUSTER_ID || true; fi; \
			rm -f /tmp/cmx-kubeconfig-$$CLUSTER_NAME; \
		}; \
		trap cleanup EXIT; \
		\
		echo "--- Building and pushing image $$IMAGE_REPO:$$IMAGE_TAG ---"; \
		docker buildx build --platform linux/amd64 \
			-t $$IMAGE_REPO:$$IMAGE_TAG --push .; \
		\
		echo "--- Creating CMX cluster $$CLUSTER_NAME ---"; \
		replicated cluster create \
			--name $$CLUSTER_NAME \
			--distribution k3s \
			--version 1.35.0 \
			--wait 10m \
			--ttl 30m; \
		\
		CLUSTER_ID=$$(replicated cluster ls --output json | jq -r ".[] | select(.name==\"$$CLUSTER_NAME\") | .id"); \
		[ -n "$$CLUSTER_ID" ] || { echo "Error: failed to get cluster ID for $$CLUSTER_NAME"; exit 1; }; \
		echo "--- Cluster ID: $$CLUSTER_ID ---"; \
		\
		echo "--- Fetching kubeconfig ---"; \
		replicated cluster kubeconfig $$CLUSTER_ID --stdout > /tmp/cmx-kubeconfig-$$CLUSTER_NAME; \
		export KUBECONFIG=/tmp/cmx-kubeconfig-$$CLUSTER_NAME; \
		\
		echo "--- Installing chart ---"; \
		kubectl create namespace clay \
			--dry-run=client -o yaml | kubectl apply -f -; \
		kubectl label namespace clay \
			app.kubernetes.io/managed-by=Helm \
			--overwrite; \
		kubectl annotate namespace clay \
			meta.helm.sh/release-name=clay \
			meta.helm.sh/release-namespace=clay \
			--overwrite; \
		kubectl create secret docker-registry ghcr-pull-secret \
			--namespace clay \
			--docker-server=ghcr.io \
			--docker-username=$$GHCR_USERNAME \
			--docker-password=$$GHCR_TOKEN; \
		helm install clay chart/clay/ \
			--namespace clay \
			--set image.tag=$$IMAGE_TAG \
			--set imagePullSecrets[0].name=ghcr-pull-secret \
			--set ingress.enabled=false \
			--set persistence.enabled=false \
			--wait \
			--timeout 5m; \
		\
		echo "--- Verifying deployment ---"; \
		kubectl rollout status deployment/clay -n clay --timeout=120s; \
		kubectl get pods -n clay; \
		echo "--- Integration test passed ---"; \
	'

APP_SLUG := xav-pottery-shop
CMX_CLUSTER := pottery-cmx-$(shell date +%s)
CMX_VERSION := $(shell date -u '+%Y.%-m.%-d-%H%M%S')-dev-$(shell git rev-parse --short HEAD)
CMX_STATE   := /tmp/pottery-cmx-state
EC_STATE    := /tmp/pottery-ec-state
EC_KEY      := /tmp/pottery-ec-key

## cmx-test: mirror CI integration-test -- build release, install via Replicated on CMX, verify (no teardown)
cmx-test:
	@for tool in replicated docker helm kubectl jq; do \
		command -v $$tool >/dev/null 2>&1 || { echo "Error: $$tool not found"; exit 1; }; \
	done
	@command -v preflight >/dev/null 2>&1 || { \
		echo "Error: preflight not found -- https://github.com/replicatedhq/troubleshoot/releases"; exit 1; }
	@test -n "$$REPLICATED_API_TOKEN" || { echo "Error: REPLICATED_API_TOKEN not set"; exit 1; }
	@VERSION=$(CMX_VERSION) \
	CLUSTER_NAME=$(CMX_CLUSTER) \
	APP_SLUG=$(APP_SLUG) \
	IMAGE_REPO=$(IMAGE_REPO) \
	CMX_STATE=$(CMX_STATE) \
	bash -ec ' \
		trap '\''git checkout replicated/clay-chart.yaml 2>/dev/null || true'\'' EXIT; \
		\
		echo "--- Building and pushing image $$IMAGE_REPO:$$VERSION ---"; \
		docker buildx build --platform linux/amd64 \
			-t $$IMAGE_REPO:$$VERSION --push .; \
		\
		echo "--- Packaging chart ---"; \
		helm registry logout registry.replicated.com 2>/dev/null || true; \
		helm dependency update chart/clay/; \
		helm lint chart/clay/; \
		rm -f replicated/clay-$$VERSION.tgz; \
		helm package chart/clay/ -d replicated \
			--version $$VERSION --app-version $$VERSION; \
		cp chart/clay/charts/cert-manager-*.tgz replicated/; \
		\
		echo "--- Creating Replicated release on Unstable ---"; \
		sed "s/chartVersion: .*/chartVersion: $$VERSION/" replicated/clay-chart.yaml \
			> /tmp/clay-chart-$$VERSION.yaml; \
		cp /tmp/clay-chart-$$VERSION.yaml replicated/clay-chart.yaml; \
		RELEASE_OUT=$$(replicated release create \
			--app $$APP_SLUG \
			--yaml-dir ./replicated \
			--promote Unstable \
			--version $$VERSION 2>&1); \
		echo "$$RELEASE_OUT"; \
		\
		echo "--- Creating customer ---"; \
		CUSTOMER_JSON=$$(replicated customer create \
			--app $$APP_SLUG \
			--name "cmx-local-$$VERSION" \
			--channel Unstable \
			--type dev \
			--expires-in 24h \
			--output json); \
		CUSTOMER_ID=$$(echo "$$CUSTOMER_JSON" | jq -r ".id"); \
		LICENSE_ID=$$(echo "$$CUSTOMER_JSON" | jq -r ".installationId"); \
		[ -n "$$CUSTOMER_ID" ] && [ "$$CUSTOMER_ID" != "null" ] \
			|| { echo "Error: failed to create customer"; exit 1; }; \
		\
		echo "--- Creating CMX cluster $$CLUSTER_NAME ---"; \
		replicated cluster create \
			--name $$CLUSTER_NAME \
			--distribution k3s \
			--version 1.35.0 \
			--instance-type r1.medium \
			--wait 10m \
			--ttl 2h; \
		CLUSTER_ID=$$(replicated cluster ls --output json \
			| jq -r ".[] | select(.name==\"$$CLUSTER_NAME\") | .id"); \
		[ -n "$$CLUSTER_ID" ] || { echo "Error: failed to get cluster ID"; exit 1; }; \
		\
		printf "CLUSTER_ID=$$CLUSTER_ID\nCUSTOMER_ID=$$CUSTOMER_ID\n" > $$CMX_STATE; \
		echo "State saved to $$CMX_STATE"; \
		\
		echo "--- Fetching kubeconfig ---"; \
		replicated cluster kubeconfig $$CLUSTER_ID --stdout > /tmp/pottery-cmx-kubeconfig; \
		export KUBECONFIG=/tmp/pottery-cmx-kubeconfig; \
		\
		echo "--- Deploying postgres pod for preflight ---"; \
		kubectl run ci-postgres --image=postgres:15 --restart=Never \
			--env=POSTGRES_PASSWORD=ci-test --env=POSTGRES_DB=ci; \
		kubectl wait pod/ci-postgres --for=condition=Ready --timeout=60s; \
		kubectl port-forward pod/ci-postgres 5432:5432 & sleep 2; \
		\
		echo "--- Running preflight checks ---"; \
		helm registry login registry.replicated.com \
			--username "cmx-local@clay.nz" \
			--password "$$LICENSE_ID"; \
		helm template clay \
			oci://registry.replicated.com/$$APP_SLUG/unstable/clay \
			--version $$VERSION \
			-f chart/clay/ci/local-test-values.yaml \
			--set postgres.managed=false \
			--set postgres.external.dsn=postgresql://postgres:ci-test@localhost/ci \
			| preflight --interactive=false -; \
		\
		echo "--- Installing chart from Replicated ---"; \
		helm install clay \
			oci://registry.replicated.com/$$APP_SLUG/unstable/clay \
			--version $$VERSION \
			--namespace clay --create-namespace \
			--set clay.image.tag=$$VERSION \
			-f chart/clay/ci/local-test-values.yaml \
			--timeout 8m; \
		\
		echo "--- Waiting for Postgres ---"; \
		for i in $$(seq 1 60); do \
			PHASE=$$(kubectl get clusters.postgresql.cnpg.io -n clay clay-postgres \
				-o jsonpath="{.status.phase}" 2>/dev/null || echo pending); \
			if echo "$$PHASE" | grep -q "Cluster in healthy state"; then \
				echo "Postgres ready"; break; \
			fi; \
			printf "."; sleep 5; \
		done; echo ""; \
		\
		echo "--- Verifying deployment ---"; \
		kubectl rollout status deployment/clay -n clay --timeout=120s; \
		kubectl get pods -n clay; \
		POD=$$(kubectl get pods -n clay -l app.kubernetes.io/name=clay \
			-o jsonpath="{.items[0].metadata.name}"); \
		kubectl exec -n clay "$$POD" -- wget -qO- http://localhost:8080/ | head -c 200; \
		echo ""; \
		\
		echo "--- Verifying cert-manager ---"; \
		for i in $$(seq 1 30); do \
			kubectl get crd clusterissuers.cert-manager.io >/dev/null 2>&1 \
				&& { echo "cert-manager CRDs ready"; break; }; \
			printf "."; sleep 5; \
		done; echo ""; \
		kubectl get clusterissuers || true; \
		kubectl get certificates -n clay || true; \
		\
		echo ""; \
		echo "=== Tests passed ==="; \
		echo "KUBECONFIG: /tmp/pottery-cmx-kubeconfig"; \
		echo "Connect:    kubectl --kubeconfig /tmp/pottery-cmx-kubeconfig get pods -n clay"; \
		echo "Teardown:   make cmx-test-teardown"; \
	'

## cmx-test-teardown: remove CMX cluster and customer left by cmx-test
cmx-test-teardown:
	@test -f $(CMX_STATE) || { echo "No state at $(CMX_STATE)"; exit 0; }
	@APP_SLUG=$(APP_SLUG) CMX_STATE=$(CMX_STATE) bash -ec ' \
		. $$CMX_STATE; \
		echo "--- Removing cluster $$CLUSTER_ID ---"; \
		replicated cluster rm "$$CLUSTER_ID" 2>/dev/null || true; \
		echo "--- Archiving customer $$CUSTOMER_ID ---"; \
		replicated customer archive --app $$APP_SLUG "$$CUSTOMER_ID" 2>/dev/null || true; \
		rm -f $$CMX_STATE /tmp/pottery-cmx-kubeconfig; \
		echo "Done"; \
	'

## ec-test: mirror CI ec-integration-test -- install EC on CMX VM, verify (no teardown)
ec-test:
	@for tool in replicated docker helm jq ssh scp; do \
		command -v $$tool >/dev/null 2>&1 || { echo "Error: $$tool not found"; exit 1; }; \
	done
	@test -n "$$REPLICATED_API_TOKEN" || { echo "Error: REPLICATED_API_TOKEN not set"; exit 1; }
	@VERSION=$(CMX_VERSION) \
	APP_SLUG=$(APP_SLUG) \
	IMAGE_REPO=$(IMAGE_REPO) \
	EC_KEY=$(EC_KEY) \
	EC_STATE=$(EC_STATE) \
	bash -ec ' \
		trap '\''git checkout replicated/clay-chart.yaml 2>/dev/null || true'\'' EXIT; \
		\
		echo "--- Building and pushing image $$IMAGE_REPO:$$VERSION ---"; \
		docker buildx build --platform linux/amd64 \
			-t $$IMAGE_REPO:$$VERSION --push .; \
		\
		echo "--- Packaging chart and creating release ---"; \
		helm dependency update chart/clay/; \
		rm -f replicated/clay-$$VERSION.tgz; \
		helm package chart/clay/ -d replicated \
			--version $$VERSION --app-version $$VERSION; \
		cp chart/clay/charts/cert-manager-*.tgz replicated/; \
		sed "s/chartVersion: .*/chartVersion: $$VERSION/" replicated/clay-chart.yaml \
			> /tmp/clay-chart-$$VERSION.yaml; \
		cp /tmp/clay-chart-$$VERSION.yaml replicated/clay-chart.yaml; \
		replicated release create \
			--app $$APP_SLUG \
			--yaml-dir ./replicated \
			--promote Unstable \
			--version $$VERSION; \
		\
		echo "--- Creating EC customer ---"; \
		CUSTOMER_JSON=$$(replicated customer create \
			--app $$APP_SLUG \
			--name "ec-local-$$VERSION" \
			--channel Unstable \
			--type dev \
			--expires-in 24h \
			--embedded-cluster-download \
			--airgap \
			--output json); \
		CUSTOMER_ID=$$(echo "$$CUSTOMER_JSON" | jq -r ".id"); \
		LICENSE_ID=$$(echo "$$CUSTOMER_JSON" | jq -r ".installationId"); \
		[ -n "$$CUSTOMER_ID" ] && [ "$$CUSTOMER_ID" != "null" ] \
			|| { echo "Error: failed to create customer"; exit 1; }; \
		\
		echo "--- Generating SSH key ---"; \
		rm -f $$EC_KEY $$EC_KEY.pub; \
		ssh-keygen -t ed25519 -C "ci@clay.nz" -f $$EC_KEY -N ""; \
		\
		echo "--- Creating CMX VM ---"; \
		VM_JSON=$$(replicated vm create \
			--distribution ubuntu \
			--version 24.04 \
			--name pottery-ec-local-$$VERSION \
			--ttl 2h \
			--ssh-public-key $$EC_KEY.pub \
			--output json); \
		VM_ID=$$(echo "$$VM_JSON" | jq -r ".[0].id"); \
		[ -n "$$VM_ID" ] && [ "$$VM_ID" != "null" ] \
			|| { echo "Error: failed to create VM"; exit 1; }; \
		\
		printf "VM_ID=$$VM_ID\nCUSTOMER_ID=$$CUSTOMER_ID\n" > $$EC_STATE; \
		echo "State saved to $$EC_STATE"; \
		\
		echo "--- Waiting for VM to be running ---"; \
		SSH_HOST=""; SSH_PORT=""; \
		for i in $$(seq 1 30); do \
			VM_DATA=$$(replicated vm ls --output json | jq ".[] | select(.id == \"$$VM_ID\")"); \
			STATUS=$$(echo "$$VM_DATA" | jq -r ".status"); \
			if [ "$$STATUS" = "running" ]; then \
				SSH_HOST=$$(echo "$$VM_DATA" | jq -r ".direct_ssh_endpoint"); \
				SSH_PORT=$$(echo "$$VM_DATA" | jq -r ".direct_ssh_port"); \
				break; \
			fi; \
			echo "VM status: $$STATUS"; sleep 10; \
		done; \
		[ -n "$$SSH_HOST" ] || { echo "Error: VM did not reach running state"; exit 1; }; \
		printf "SSH_HOST=$$SSH_HOST\nSSH_PORT=$$SSH_PORT\n" >> $$EC_STATE; \
		\
		SSH="ssh -i $$EC_KEY -o StrictHostKeyChecking=no -p $$SSH_PORT"; \
		SCP="scp -i $$EC_KEY -o StrictHostKeyChecking=no -P $$SSH_PORT"; \
		\
		echo "--- Uploading config values ---"; \
		$$SCP replicated/local-test-config-values.yaml ci@$$SSH_HOST:~/config-values.yaml; \
		\
		echo "--- Downloading and installing EC ---"; \
		$$SSH -o ServerAliveInterval=30 -o ServerAliveCountMax=40 ci@$$SSH_HOST \
			"curl -f https://replicated.app/embedded/$$APP_SLUG/unstable \
				-H \"Authorization: $$LICENSE_ID\" \
				-o ~/$$APP_SLUG.tgz && \
			tar -xzf ~/$$APP_SLUG.tgz && \
			sudo ~/$$APP_SLUG install \
				--license ~/license.yaml \
				--config-values ~/config-values.yaml \
				--headless \
				--installer-password local-test-password \
				--yes"; \
		\
		echo "--- Verifying installation ---"; \
		$$SSH -o ServerAliveInterval=30 -o ServerAliveCountMax=20 ci@$$SSH_HOST \
			"K0S=\$$(find /usr/local/bin /var/lib/$$APP_SLUG/bin /usr/bin -name k0s -type f 2>/dev/null | head -1); \
			[ -n \"\$$K0S\" ] || { echo k0s not found; exit 1; }; \
			NS=$$APP_SLUG; \
			echo === Nodes ===; \
			sudo \$$K0S kubectl get nodes; \
			echo === Waiting for clay deployment ===; \
			sudo \$$K0S kubectl rollout status deployment/clay -n \$$NS --timeout=300s; \
			echo === Waiting for Postgres ===; \
			for i in \$$(seq 1 60); do \
				PHASE=\$$(sudo \$$K0S kubectl get clusters.postgresql.cnpg.io -n \$$NS clay-postgres \
					-o jsonpath={.status.phase} 2>/dev/null || echo pending); \
				echo \"\$$PHASE\" | grep -q \"Cluster in healthy state\" && { echo Postgres ready; break; }; \
				printf .; sleep 5; \
			done; echo; \
			sudo \$$K0S kubectl get pods -A"; \
		\
		echo ""; \
		echo "=== EC test passed ==="; \
		echo "SSH:      ssh -i $$EC_KEY -p $$SSH_PORT ci@$$SSH_HOST"; \
		echo "Teardown: make ec-test-teardown"; \
	'

## ec-test-teardown: remove CMX VM and customer left by ec-test
ec-test-teardown:
	@test -f $(EC_STATE) || { echo "No state at $(EC_STATE)"; exit 0; }
	@APP_SLUG=$(APP_SLUG) EC_STATE=$(EC_STATE) bash -ec ' \
		. $$EC_STATE; \
		echo "--- Removing VM $$VM_ID ---"; \
		replicated vm rm "$$VM_ID" 2>/dev/null || true; \
		echo "--- Archiving customer $$CUSTOMER_ID ---"; \
		replicated customer archive --app $$APP_SLUG "$$CUSTOMER_ID" 2>/dev/null || true; \
		rm -f $$EC_STATE $(EC_KEY) $(EC_KEY).pub /tmp/pottery-ec-license.yaml; \
		echo "Done"; \
	'

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
