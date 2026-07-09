# AGENTS.md

All work happens through PRs. Never push directly to main.

This workspace is for `xavpaice/pottery-shop`. Always read and follow the
instructions in the upstream `CLAUDE.md` before starting any work in that repo.

## Repo

- `xavpaice/pottery-shop` - Pottery Shop

## Working Style

- Prefer hard errors over fallbacks or silent defaults
- Use proper error handling and retry logic where needed
- Keep changes scoped to the requested behavior
- Add or update tests when behavior changes
- Communicate progress on long-running tasks

## Build & Test

- `make build` - Build the `pottery-server` binary
- `make test` - Run unit and integration tests (uses Testcontainers)
- `make test-verbose` - Run tests with verbose output
- `make run-local` - Build and run the server locally
- `make helm-lint` - Lint the Helm chart
- `make docker` - Build the Docker image

## Git And GitHub CLI

Use the preinstalled Git credential helper for all authentication. Do not hunt
for tokens, read `gh auth token`, or embed credentials in remote URLs. The
environment is configured so `git` and `gh` obtain credentials through the
helper.
