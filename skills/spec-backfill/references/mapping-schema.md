# Mapping Schema

Permanent file: `specs/spec-mapping.yaml`

Tracks code↔spec relationships, classification, and freshness (via SHA).

```yaml
version: 1
repository: "git@github.com:org/repo.git"
last_updated: "2024-01-15T14:22:00Z"

code_classification:
  # path → {type: functional | architectural | utility, confidence: confirmed | inferred}
  "src/billing/invoice.py": {type: functional, confidence: confirmed}
  "src/lib/kafka_client.py": {type: architectural, confidence: confirmed}
  "src/utils/formatting.py": {type: utility, confidence: inferred}

mappings:
  # code_path → spec mapping
  # Staleness: compare code_sha and spec_sha against current HEAD
  #   code_sha stale, spec_sha current → code-ahead-of-spec
  #   code_sha current, spec_sha stale → spec edited (intentional or drift?)
  #   both stale → full reconciliation needed
  "src/billing/invoice.py":
    spec_path: "specs/functional/1.2.1.md"
    spec_title: "Invoicing"
    code_sha: a1b2c3d
    spec_sha: x9y8z7w
    status: aligned  # aligned | gap | conflict (stale is computed from SHA comparison, not stored)
    last_verified: "2024-01-15"
    confidence: confirmed  # confirmed | inferred-pending-review
    conflict_details: null
  "src/notifications/email.py":
    spec_path: null
    spec_title: null
    code_sha: d4e5f6a
    spec_sha: null
    status: gap
    last_verified: "2024-01-15"
    confidence: inferred-pending-review
    inferred_feature: "Email notifications"
  "src/auth/login.py":
    spec_path: "specs/functional/1.1.1.md"
    spec_title: "User Login"
    code_sha: b7c8d9e
    spec_sha: f3g4h5i
    status: conflict
    last_verified: "2024-01-15"
    confidence: confirmed
    conflict_details: "Code has MFA, spec doesn't mention it"

specs:
  # spec_path → metadata
  "specs/functional/1.2.1.md":
    title: "Invoicing"
    status: active  # draft | active | deprecated | removed
    last_verified: "2024-01-15"
    linked_adrs: ["ADR-0012", "ADR-0015"]
    code_paths: ["src/billing/invoice.py", "src/billing/line_items.py"]
  "specs/build/1.2.md":
    title: "Billing Epic"
    status: active
    last_verified: "2024-01-15"
    linked_adrs: ["ADR-0012"]
    child_specs: ["specs/build/1.2.1.md", "specs/build/1.2.2.md"]

archive:
  # archived_path → metadata
  "specs/_archive/old-billing-spec.md":
    original_path: "docs/billing.md"
    archived_at: "2024-01-15"
    reason: "Migrated to specs/functional/1.2.x"
    content_verified: true
```
