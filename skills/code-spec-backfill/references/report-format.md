# Report Format

For unresolvable gaps only. Backfilled contracts are written directly to source files.

Output to `code-spec-gaps.md` or user-specified file:

```
# Code Spec Backfill Report
Scope: <description>
Targets analyzed: N | Backfilled: X | Unresolvable gaps: Y | Skipped (adequate): Z

## Backfilled

Summary of contracts added (review these):
- file:function ⚠️ — what was added (code-only inference, no call sites or tests)

## Unresolvable Gaps

### file:function

**Gap**: <what's unclear>
**Misuse scenario**: <caller_assumption> × <actual_behavior> → <consequence>
**Victim**: caller-confusion | maintainer-trap | silent-corruption
**Why unresolvable**: <what's ambiguous — needs human decision>
```

## Victim Categories

- **caller-confusion**: API consumers make wrong assumptions
- **maintainer-trap**: Future editors will break it
- **silent-corruption**: Wrong data propagates without errors
