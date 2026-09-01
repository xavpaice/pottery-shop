#!/usr/bin/env bash
set -euo pipefail

repo="${1:-}"
base_branch="${2:-}"
deps_branch="${3:-}"
commit_message="${4:-[deps] Dependency update}"

if [ -z "$repo" ] || [ -z "$base_branch" ] || [ -z "$deps_branch" ]; then
  echo "Usage: $0 <owner/repo> <base-branch> <deps-branch> [commit-message]" >&2
  exit 1
fi

repo_name="$(basename "$repo")"

if [ ! -d "$repo_name" ]; then
  gh repo clone "$repo" "$repo_name"
fi

cd "$repo_name"

# Ensure git identity is set (required when creating commits later).
git config user.email "elasticclaw@factory" || true
git config user.name "ElasticClaw Factory" || true

# Refresh base branch and recreate the dependency branch from it.
git fetch origin
git checkout "$base_branch"
git reset --hard "origin/${base_branch}"
git branch -D "$deps_branch" 2>/dev/null || true
git checkout -b "$deps_branch"

# Push the dependency branch to origin. This allows the dependency update and PR
# steps to work against a remote branch.
git push -f origin "$deps_branch"

# Output the branch so downstream steps can read it if needed.
echo "{"\"branch\"": "\"$deps_branch\""}"
