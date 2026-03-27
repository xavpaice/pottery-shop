.PHONY: build test test-verbose clean run

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

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
