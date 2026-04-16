.PHONY: build test test-verbose clean run run-local run-stop docker helm-lint lint integration-test

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

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
