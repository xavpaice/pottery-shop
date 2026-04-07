.PHONY: build test test-verbose clean run docker helm-lint lint integration-test

BINARY := pottery-server
GO := go

## build: compile the server binary
build:
	CGO_ENABLED=1 $(GO) build -o $(BINARY) ./cmd/server

## test: run all tests
test:
	CGO_ENABLED=1 $(GO) test ./...

## test-verbose: run all tests with verbose output
test-verbose:
	CGO_ENABLED=1 $(GO) test -v ./...

## test-coverage: run tests with coverage report
test-coverage:
	CGO_ENABLED=1 $(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: remove build artifacts
clean:
	rm -f $(BINARY) coverage.out coverage.html
	rm -f pottery.db

## run: build and run the server
run: build
	./$(BINARY)

## tidy: tidy and verify module dependencies
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## helm-lint: lint the Helm chart
helm-lint:
	helm lint chart/clay/

## lint: run all linters
lint: helm-lint

## docker: build Docker image
docker:
	docker build -t ghcr.io/xavpaice/pottery-shop:latest .

## deploy: apply all Kubernetes manifests
deploy:
	kubectl apply -f k8s/

IMAGE_REPO := ghcr.io/xavpaice/pottery-shop
IMAGE_TAG := test-$(shell git rev-parse --short HEAD)
CLUSTER_NAME := pottery-integration-$(shell date +%s)

## integration-test: build image, create CMX cluster, install chart, verify, teardown
integration-test:
	@command -v replicated >/dev/null 2>&1 || { echo "Error: replicated CLI not installed. See https://docs.replicated.com/reference/replicated-cli-installing"; exit 1; }
	@test -n "$$REPLICATED_API_TOKEN" || { echo "Error: REPLICATED_API_TOKEN not set"; exit 1; }
	@test -n "$$GHCR_TOKEN" || { echo "Error: GHCR_TOKEN not set"; exit 1; }
	@CLUSTER_NAME=$(CLUSTER_NAME) \
	IMAGE_REPO=$(IMAGE_REPO) \
	IMAGE_TAG=$(IMAGE_TAG) \
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
			--docker-username=xavpaice \
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
