# Session State Schema

Persistent file: `specs/spec-backfill-state.yaml`

Tracks processing cursor so work resumes across conversation boundaries.

```yaml
version: 1
started_at: "2024-01-15T10:30:00Z"
last_updated: "2024-01-15T14:22:00Z"

processing_cursor:
  step: "mapping"  # classification | mapping | gap_analysis | generation | verification | archiving
  current_path: "src/notifications/email.py"
  substep: "awaiting_user_input"

configuration:
  functional_roots: ["src/"]
  exclude_patterns: ["**/tests/**", "**/migrations/**"]
  spec_output_dir: "specs"
  require_confirmation: true
```
