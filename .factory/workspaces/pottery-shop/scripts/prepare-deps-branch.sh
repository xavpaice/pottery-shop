#!/usr/bin/env bash
# Prepare a dependency-update branch for an ElasticClaw Factory workflow.
#
# Usage: prepare-deps-branch.sh <owner/repo> <base_branch> <expected_branch> <pr_title>
#
# The script checks out the repository into a subdirectory named after the repo
# (e.g. replicatedhq/troubleshoot.sh -> ./troubleshoot.sh), then:
#   1. Fetches the latest base branch and resets to it.
#   2. Looks for an open PR matching the expected branch name or the PR title.
#   3. If found, fetches the PR branch, checks it out as <expected_branch>,
#      and rebases it onto the latest base branch.
#   4. If not found, deletes any stale local/remote <expected_branch> and
#      creates a fresh branch from the base branch.
#
# Prints a single JSON object as the final line of stdout for workflow gates.

set -euo pipefail

repo="${1:-}"
base_branch="${2:-}"
expected_branch="${3:-}"
pr_title="${4:-}"

if [[ -z "$repo" || -z "$base_branch" || -z "$expected_branch" || -z "$pr_title" ]]; then
    echo "Usage: $0 <owner/repo> <base_branch> <expected_branch> <pr_title>" >&2
    exit 1
fi

repo_name="${repo#*/}"
if [[ ! -d "$repo_name" ]]; then
    git clone "https://github.com/${repo}.git" "$repo_name"
fi
cd "$repo_name"

emit_json() {
    python3 - "$@" <<PY
import json
import sys
print(json.dumps(dict(arg.split('=', 1) for arg in sys.argv[1:])))
PY
}

fail() {
    local reason="$1"
    emit_json \
        "status=error" \
        "reason=$reason" \
        "repo=$repo" \
        "base_branch=$base_branch" \
        "expected_branch=$expected_branch" \
        "pr_title=$pr_title"
    exit 1
}

# Ensure the base branch is up to date.
git fetch origin "$base_branch" || fail "failed to fetch origin/$base_branch"
git checkout "$base_branch" || fail "failed to checkout $base_branch"
git reset --hard "origin/$base_branch" || fail "failed to reset to origin/$base_branch"

pr_number=""
pr_branch=""

# First, search by the expected branch name.
pr_json=$(gh pr list --repo "$repo" --head "$expected_branch" --state open --json number,headRefName,title --jq '.[0]' 2>/dev/null || true)
if [[ -n "$pr_json" && "$pr_json" != "null" && "$pr_json" != "{}" ]]; then
    pr_number=$(echo "$pr_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("number",""))')
    pr_branch=$(echo "$pr_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("headRefName",""))')
fi

# Fall back to searching by PR title.
if [[ -z "$pr_number" ]]; then
    pr_json=$(gh pr list --repo "$repo" --search "$pr_title" --state open --json number,headRefName,title --jq '.[0]' 2>/dev/null || true)
    if [[ -n "$pr_json" && "$pr_json" != "null" && "$pr_json" != "{}" ]]; then
        pr_number=$(echo "$pr_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("number",""))')
        pr_branch=$(echo "$pr_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("headRefName",""))')
    fi
fi

if [[ -n "$pr_number" && -n "$pr_branch" ]]; then
    # Existing PR found: rebase its branch onto the latest base branch.
    git fetch origin "$pr_branch" || fail "failed to fetch existing PR branch $pr_branch"

    if git rev-parse --verify "$expected_branch" >/dev/null 2>&1; then
        git checkout "$expected_branch" || fail "failed to checkout local $expected_branch"
        git reset --hard "origin/$pr_branch" || fail "failed to reset to origin/$pr_branch"
    else
        git checkout -b "$expected_branch" "origin/$pr_branch" || fail "failed to create $expected_branch from origin/$pr_branch"
    fi

    git rebase "origin/$base_branch" || fail "failed to rebase $expected_branch onto origin/$base_branch"

    emit_json \
        "status=ready" \
        "action=rebased" \
        "branch=$expected_branch" \
        "pr_number=$pr_number" \
        "pr_branch=$pr_branch" \
        "repo=$repo" \
        "base_branch=$base_branch" \
        "pr_title=$pr_title"
else
    # No open PR: start fresh.
    if git rev-parse --verify "$expected_branch" >/dev/null 2>&1; then
        git branch -D "$expected_branch" || true
    fi

    if git ls-remote --heads origin "$expected_branch" | grep -q "$expected_branch"; then
        git push origin --delete "$expected_branch" || fail "failed to delete remote $expected_branch"
    fi

    git checkout -b "$expected_branch" || fail "failed to create $expected_branch"

    emit_json \
        "status=ready" \
        "action=created" \
        "branch=$expected_branch" \
        "pr_number=" \
        "pr_branch=" \
        "repo=$repo" \
        "base_branch=$base_branch" \
        "pr_title=$pr_title"
fi
