#!/usr/bin/env bash
# Update a single entry in spec-mapping.yaml.
#
# Usage:
#   update-mapping.sh <mapping-file> <code-path> [options]
#
# Options:
#   --status STATUS           aligned | gap | conflict
#   --code-sha SHA            Set code SHA (or "auto" to read from git)
#   --spec-sha SHA            Set spec SHA (or "auto" to read from git)
#   --spec-path PATH          Set linked spec path
#   --spec-title TITLE        Set spec title
#   --confidence CONF         confirmed | inferred-pending-review
#   --conflict-details TEXT   Set conflict details (or "null" to clear)
#   --classify TYPE           Set code_classification entry (functional | architectural | utility)
#   --remove                  Remove entry from mapping
#
# Examples:
#   update-mapping.sh specs/spec-mapping.yaml src/billing/invoice.py --status aligned --code-sha auto --spec-sha auto
#   update-mapping.sh specs/spec-mapping.yaml src/new/feature.py --status gap --code-sha auto --classify functional
#   update-mapping.sh specs/spec-mapping.yaml src/removed/old.py --remove
#
# Exit codes:
#   0 — success
#   1 — error

set -euo pipefail

if ! command -v yq &>/dev/null; then
    echo "ERROR: yq (mikefarah/yq) is required but not installed" >&2
    exit 1
fi

if [[ $# -lt 2 ]]; then
    echo "Usage: update-mapping.sh <mapping-file> <code-path> [options]" >&2
    exit 1
fi

MAPPING="$1"
CODE_PATH="$2"
shift 2

# Defaults
STATUS=""
CODE_SHA=""
SPEC_SHA=""
SPEC_PATH=""
SPEC_TITLE=""
CONFIDENCE=""
CONFLICT_DETAILS=""
CLASSIFY=""
REMOVE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --status)         STATUS="$2"; shift 2 ;;
        --code-sha)       CODE_SHA="$2"; shift 2 ;;
        --spec-sha)       SPEC_SHA="$2"; shift 2 ;;
        --spec-path)      SPEC_PATH="$2"; shift 2 ;;
        --spec-title)     SPEC_TITLE="$2"; shift 2 ;;
        --confidence)     CONFIDENCE="$2"; shift 2 ;;
        --conflict-details) CONFLICT_DETAILS="$2"; shift 2 ;;
        --classify)       CLASSIFY="$2"; shift 2 ;;
        --remove)         REMOVE=true; shift ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Initialize mapping file if it doesn't exist
if [[ ! -f "$MAPPING" ]]; then
    cat > "$MAPPING" <<'INIT'
version: 1
last_updated: ""
code_classification: {}
mappings: {}
specs: {}
archive: {}
INIT
fi

# Remove entry
if $REMOVE; then
    export YQ_PATH="$CODE_PATH"
    yq -i 'del(.mappings[env(YQ_PATH)])' "$MAPPING"
    yq -i 'del(.code_classification[env(YQ_PATH)])' "$MAPPING"
    export YQ_VAL="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    yq -i '.last_updated = env(YQ_VAL)' "$MAPPING"
    echo "Removed: $CODE_PATH"
    exit 0
fi

# Resolve "auto" SHAs
if [[ "$CODE_SHA" == "auto" ]]; then
    if git ls-files --error-unmatch "$CODE_PATH" &>/dev/null; then
        CODE_SHA=$(git log -1 --format=%h -- "$CODE_PATH" 2>/dev/null || echo "")
    else
        echo "WARNING: $CODE_PATH not tracked by git, cannot auto-resolve code SHA" >&2
        CODE_SHA=""
    fi
fi

if [[ "$SPEC_SHA" == "auto" ]]; then
    # Read spec_path from existing entry or from --spec-path
    resolve_spec_path="$SPEC_PATH"
    if [[ -z "$resolve_spec_path" ]]; then
        export YQ_PATH="$CODE_PATH"
        resolve_spec_path=$(yq '.mappings[env(YQ_PATH)].spec_path // ""' "$MAPPING" 2>/dev/null)
    fi
    if [[ -n "$resolve_spec_path" && "$resolve_spec_path" != "null" && -f "$resolve_spec_path" ]]; then
        if git ls-files --error-unmatch "$resolve_spec_path" &>/dev/null; then
            SPEC_SHA=$(git log -1 --format=%h -- "$resolve_spec_path" 2>/dev/null || echo "")
        fi
    else
        echo "WARNING: no spec path to auto-resolve spec SHA" >&2
        SPEC_SHA=""
    fi
fi

# Ensure mapping entry exists
export YQ_PATH="$CODE_PATH"
existing=$(yq '.mappings[env(YQ_PATH)] // ""' "$MAPPING" 2>/dev/null)
if [[ -z "$existing" || "$existing" == "null" ]]; then
    yq -i '.mappings[env(YQ_PATH)] = {}' "$MAPPING"
fi

# Apply updates using env() binding for safe value interpolation
TODAY=$(date -u +%Y-%m-%d)

if [[ -n "$STATUS" ]]; then
    export YQ_VAL="$STATUS"
    yq -i '.mappings[env(YQ_PATH)].status = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$CODE_SHA" ]]; then
    export YQ_VAL="$CODE_SHA"
    yq -i '.mappings[env(YQ_PATH)].code_sha = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$SPEC_SHA" ]]; then
    export YQ_VAL="$SPEC_SHA"
    yq -i '.mappings[env(YQ_PATH)].spec_sha = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$SPEC_PATH" ]]; then
    export YQ_VAL="$SPEC_PATH"
    yq -i '.mappings[env(YQ_PATH)].spec_path = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$SPEC_TITLE" ]]; then
    export YQ_VAL="$SPEC_TITLE"
    yq -i '.mappings[env(YQ_PATH)].spec_title = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$CONFIDENCE" ]]; then
    export YQ_VAL="$CONFIDENCE"
    yq -i '.mappings[env(YQ_PATH)].confidence = env(YQ_VAL)' "$MAPPING"
fi

if [[ -n "$CONFLICT_DETAILS" ]]; then
    if [[ "$CONFLICT_DETAILS" == "null" ]]; then
        yq -i '.mappings[env(YQ_PATH)].conflict_details = null' "$MAPPING"
    else
        export YQ_VAL="$CONFLICT_DETAILS"
        yq -i '.mappings[env(YQ_PATH)].conflict_details = env(YQ_VAL)' "$MAPPING"
    fi
fi

# Always update last_verified and last_updated
export YQ_VAL="$TODAY"
yq -i '.mappings[env(YQ_PATH)].last_verified = env(YQ_VAL)' "$MAPPING"
export YQ_VAL="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
yq -i '.last_updated = env(YQ_VAL)' "$MAPPING"

# Classification
if [[ -n "$CLASSIFY" ]]; then
    export YQ_VAL="$CLASSIFY"
    yq -i '.code_classification[env(YQ_PATH)] = {"type": env(YQ_VAL), "confidence": "confirmed"}' "$MAPPING"
fi

echo "Updated: $CODE_PATH"
