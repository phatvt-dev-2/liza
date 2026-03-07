# Clean Code Plan

Apply clean-code skill (full-file mode) to all 122 non-test .go files across 22 packages in `internal/` and `cmd/`.

## Stats

- Total files: 122
- Total packages: 22
- Total tasks: 26 (batches of ≤5 files)
- Dependencies: none (tasks are independent)

## Task Breakdown

### Small packages (grouped into batches of 5)

| # | Files | Packages |
|---|-------|----------|
| 1 | cmd/liza/main.go, cmd/liza-mcp/main.go, internal/db/{blackboard,doc,watcher}.go | cmd/*, internal/db |
| 2 | internal/models/{diagnostics,doc,lease,state}.go, internal/analysis/patterns.go | internal/models, internal/analysis |
| 3 | internal/errors/{doc,errors}.go, internal/log/{doc,logger}.go, internal/roles/roles.go | internal/errors, internal/log, internal/roles |
| 4 | internal/filelock/{doc,errors,filelock,metrics}.go, internal/statevalidate/validate.go | internal/filelock, internal/statevalidate |
| 5 | internal/mcp/{handlers,server}.go, internal/paths/{doc,paths}.go, internal/git/worktree.go | internal/mcp, internal/paths, internal/git |
| 6 | internal/mcp/protocol/{errors,stdio,testing,types}.go, internal/embedded/embedded.go | internal/mcp/protocol, internal/embedded |
| 7 | internal/pipeline/{config,resolver}.go, internal/prompts/{builder,templates}.go, internal/identity/resolver.go | internal/pipeline, internal/prompts, internal/identity |

### internal/agent (10 files → 2 tasks)

| # | Files |
|---|-------|
| 8 | claiming.go, heartbeat.go, logging.go, output.go, prompt.go |
| 9 | registration.go, supervisor.go, systemctl.go, waitforwork.go, workdetection.go |

### internal/commands (39 files → 8 tasks)

| # | Files |
|---|-------|
| 10 | add_task.go, analyze.go, claim_task.go, clear_stale_review_claims.go, delete_agent.go |
| 11 | delete_task.go, doc.go, format.go, handoff.go, init.go |
| 12 | inspect.go, inspect_agents.go, inspect_anomalies.go, inspect_field.go, inspect_metrics.go |
| 13 | inspect_tasks.go, inspect_time.go, mark_blocked.go, pause.go, proceed.go |
| 14 | recover_agent.go, recover_task.go, release_claim.go, resume.go, setup.go |
| 15 | sprint_checkpoint.go, start.go, status.go, stop.go, submit_review.go |
| 16 | submit_verdict.go, supersede_task.go, templates.go, update_sprint_metrics.go, validate.go |
| 17 | watch.go, wt_create.go, wt_delete.go, wt_merge.go |

### internal/ops (32 files → 7 tasks)

| # | Files |
|---|-------|
| 18 | add_tasks.go, advance_sprint.go, analyze.go, claim_reviewer_task.go, claim_task.go |
| 19 | clear_stale_review_claims.go, delete_agent.go, delete_task.go, doc.go, handoff.go |
| 20 | helpers.go, iteration_limits.go, mark_blocked.go, mode_change.go, pipeline_ops.go |
| 21 | precondition_error.go, proceed.go, recover_agent.go, recover_task.go, release_claim.go |
| 22 | resume_handoff.go, set_task_output.go, sprint_checkpoint.go, submit_review.go, submit_verdict.go |
| 23 | supersede_task.go, test_detection.go, update_sprint_metrics.go, write_checkpoint.go, wt_create.go |
| 24 | wt_delete.go, wt_merge.go |

### internal/testhelpers (6 files → 2 tasks)

| # | Files |
|---|-------|
| 25 | assertions.go, fixtures.go, git.go, ops_testing.go, setup.go |
| 26 | utils.go |

## Per-task contract

Each task:
- **Skill**: clean-code (full-file mode, Liza behavior)
- **Language profile**: skills/clean-code/languages/go.md
- **done_when**: clean-code skill applied to all files in scope, pre-commit passes on touched files, all tests pass
- **spec_ref**: clean-code.md
- **Dependencies**: none (independent batches)
