#!/usr/bin/env bash
# Detect stale mappings by comparing stored SHAs against current HEAD.
#
# Usage: detect-stale.sh [mapping-file]
# Default: specs/spec-mapping.yaml
#
# Output (one line per mapping entry):
#   CURRENT    <path>
#   CODE_AHEAD <path>                  — code changed, spec didn't
#   SPEC_EDITED <path>                 — spec changed, code didn't
#   BOTH_STALE <path>                  — both changed, needs reconciliation
#   DELETED    <path>                  — file removed from disk
#   NEW        <path>                  — file on disk, not in mapping
#
# Exit codes:
#   0 — completed (may have stale/new entries)
#   1 — error (missing yq, missing mapping, etc.)

set -euo pipefail

MAPPING="${1:-specs/spec-mapping.yaml}"

if ! command -v yq &>/dev/null; then
    echo "ERROR: yq (mikefarah/yq) is required but not installed" >&2
    exit 1
fi

if [[ ! -f "$MAPPING" ]]; then
    echo "NO_MAPPING"
    exit 0
fi

# --- Check existing mappings ---
# Extract all entries in one yq call to avoid per-entry process spawns
entries=$(yq -o=json '.mappings // {}' "$MAPPING" 2>/dev/null)
paths=$(echo "$entries" | yq 'keys | .[]' 2>/dev/null) || true

for path in $paths; do
    if [[ ! -f "$path" ]]; then
        echo "DELETED $path"
        continue
    fi

    export YQ_PATH="$path"
    stored_code_sha=$(echo "$entries" | yq '.[env(YQ_PATH)].code_sha // ""')
    stored_spec_sha=$(echo "$entries" | yq '.[env(YQ_PATH)].spec_sha // ""')
    spec_path=$(echo "$entries" | yq '.[env(YQ_PATH)].spec_path // ""')

    # Current code SHA from git
    current_code_sha=""
    if git ls-files --error-unmatch "$path" &>/dev/null; then
        current_code_sha=$(git log -1 --format=%h -- "$path" 2>/dev/null || echo "")
    fi

    # Current spec SHA from git
    current_spec_sha=""
    if [[ -n "$spec_path" && "$spec_path" != "null" && -f "$spec_path" ]]; then
        if git ls-files --error-unmatch "$spec_path" &>/dev/null; then
            current_spec_sha=$(git log -1 --format=%h -- "$spec_path" 2>/dev/null || echo "")
        fi
    fi

    # Compare
    code_stale=false
    spec_stale=false

    if [[ -n "$stored_code_sha" && "$stored_code_sha" != "null" && "$stored_code_sha" != "$current_code_sha" ]]; then
        code_stale=true
    fi

    if [[ -n "$stored_spec_sha" && "$stored_spec_sha" != "null" && -n "$current_spec_sha" && "$stored_spec_sha" != "$current_spec_sha" ]]; then
        spec_stale=true
    fi

    if $code_stale && $spec_stale; then
        echo "BOTH_STALE $path"
    elif $code_stale; then
        echo "CODE_AHEAD $path"
    elif $spec_stale; then
        echo "SPEC_EDITED $path"
    else
        echo "CURRENT $path"
    fi
done

# --- Detect new files (in git, not in mapping) ---
# Infer roots from existing mapping paths (no external config dependency)
roots=$(echo "$paths" | xargs -I{} dirname {} 2>/dev/null | sort -u) || true

# Pre-load all mapped paths into an associative array for O(1) lookup
declare -A mapped_paths
for path in $paths; do
    mapped_paths["$path"]=1
done

for root in $roots; do
    [[ -d "$root" ]] || continue
    git ls-files "$root" 2>/dev/null | while IFS= read -r file; do
        # Skip non-source files (tests, migrations, configs)
        case "$file" in
            *test*|*_test.*|*_spec.*|*.spec.*|*migration*|*.yaml|*.yml|*.json|*.toml|*.md) continue ;;
        esac
        # Check if already in mapping
        if [[ -z "${mapped_paths[$file]+x}" ]]; then
            echo "NEW $file"
        fi
    done
done
