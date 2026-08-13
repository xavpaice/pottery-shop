#!/usr/bin/env bash
# Detect whether Go dependency updates produced changes in a repository.
#
# Usage: detect-go-changes.sh <repo_directory> [<subdirectory>]
#
# The script expects to run from the workspace root. It:
#   1. Changes into <repo_directory>.
#   2. Checks if go.mod, go.sum, or vendor/ changed in the working tree,
#      optionally scoped to <subdirectory>.
#   3. If go.mod/go.sum changed and a vendor/ directory exists at the module
#      root, runs `go mod vendor` to keep the vendor tree in sync.
#   4. Reports the current set of changed files as JSON so the workflow can
#      decide whether to run validation or terminate with no changes.

set -euo pipefail

repo_dir="${1:-}"
subdir="${2:-}"

if [[ -z "$repo_dir" ]]; then
    echo "Usage: $0 <repo_directory> [<subdirectory>]" >&2
    exit 1
fi

cd "$repo_dir"

module_dir="."
if [[ -n "$subdir" ]]; then
    module_dir="$subdir"
fi

# If go.mod/go.sum changed and a vendor directory exists at the module root,
# keep it in sync.
go_changes=$(git status --porcelain | grep -E "^${subdir:+$subdir/}go\.(mod|sum)$" || true)
if [[ -n "$go_changes" && -d "$module_dir/vendor" ]]; then
    (cd "$module_dir" && go mod vendor)
fi

# Report the current set of changed files, scoped to the subdirectory if given.
python3 - "$repo_dir" "$subdir" <<PY
import json
import subprocess
import sys

repo_dir, subdir = sys.argv[1:3]
prefix = f"{subdir}/" if subdir else ""

files = sorted(set(
    line.split(maxsplit=1)[1]
    for line in subprocess.check_output(["git", "status", "--porcelain", "."], text=True).splitlines()
    if line.strip() and line.split(maxsplit=1)[1].startswith(prefix)
))
print(json.dumps({
    "status": "has_changes" if files else "no_changes",
    "files_changed": files,
}))
PY
