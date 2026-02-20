# Report Format

Output to stdout after each run. Mapping updates are written to `specs/spec-mapping.yaml`.

```
# Spec Backfill Report
Scope: <description>
Mappings analyzed: N | Aligned: W | Stale: X | Gaps: Y | Conflicts: Z

## Stale (changed since last verification)

- src/billing/invoice.py → specs/functional/1.2.1.md — code changed (code_sha stale)
- specs/functional/1.1.1.md ← src/auth/login.py — spec edited (spec_sha stale)
- src/orders/fulfillment.py → specs/functional/1.3.1.md — both changed, needs reconciliation

## Gaps (code without spec)

- src/reports/export.py — inferred feature: CSV/PDF export
- src/webhooks/stripe.py — business intent unknown

## Conflicts

- src/auth/login.py → specs/functional/1.1.1.md
  Type: code-ahead | spec-ahead | semantic-mismatch
  Details: Code has MFA, spec doesn't mention it
```

## Conflict Types

- **code-ahead**: Code has functionality not in spec
- **spec-ahead**: Spec describes feature not yet implemented
- **semantic-mismatch**: Both exist but describe different behavior
- **stale-spec**: Spec describes removed functionality
