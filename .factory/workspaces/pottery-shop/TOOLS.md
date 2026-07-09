# TOOLS.md - Local Notes

## Long-Running Tasks

The command timeout can be shorter than builds, tests, and deploys. For tasks
that may take longer, use background processes and poll for progress.

### Pattern For Long Tasks

1. Start in the background or with a short yield interval
2. Poll for status
3. Check logs
4. Report progress periodically

### Examples

- `make build`
- `make test`
- `make test-integration`
- `go build ./...`
- Docker builds

### Do

- Start long commands in the background when appropriate
- Poll every 30-60 seconds
- Update the chat with useful progress
- Stop stuck processes if they hang too long

### Do Not

- Run blocking commands that take more than a few minutes without polling
- Go silent during long tasks
- Assume a command is fast if you have not timed it

## Git And GitHub CLI

Use the preinstalled Git credential helper for all authentication. Do not hunt
for tokens, read `gh auth token`, or embed credentials in remote URLs. The
environment is configured so `git` and `gh` obtain credentials through the
helper.
