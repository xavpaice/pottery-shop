#!/usr/bin/env bash
set -euo pipefail

repo_dir="${1:-}"

if [ -z "$repo_dir" ]; then
  echo "Usage: $0 <repo-directory>" >&2
  exit 1
fi

cd "$repo_dir"

if git diff --quiet HEAD -- go.mod go.sum go.work go.work.sum vendor 2>/dev/null; then
  echo '{"status":"no_changes"}'
else
  echo '{"status":"has_changes"}'
fi
