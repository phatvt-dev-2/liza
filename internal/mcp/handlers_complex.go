package mcp

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// handleWtCreate implements the liza_wt_create tool
// Maps to: liza wt-create
func (s *Server) handleWtCreate(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	fresh, _ := params["fresh"].(bool)

	result, err := ops.CreateWorktree(s.projectRoot, taskID, fresh)
	if err != nil {
		return nil, fmt.Errorf("wt-create failed: %w", err)
	}

	msg := fmt.Sprintf("Worktree created for task %s", taskID)
	if result.AlreadyExisted {
		msg = fmt.Sprintf("Worktree already exists for task %s", taskID)
	}
	return textResult(appendWarnings(msg, result.Warnings))
}

// handleWtDelete implements the liza_wt_delete tool
// Maps to: liza wt-delete
func (s *Server) handleWtDelete(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	result, err := ops.DeleteWorktree(s.projectRoot, taskID)
	if err != nil {
		return nil, fmt.Errorf("wt-delete failed: %w", err)
	}

	if !result.Existed {
		return textResult(fmt.Sprintf("No worktree for task %s", taskID))
	}

	return textResult(appendWarnings(
		fmt.Sprintf("Worktree deleted for task %s", taskID),
		result.Warnings,
	))
}

// handleWtMerge implements the liza_wt_merge tool
// Maps to: liza wt-merge
func (s *Server) handleWtMerge(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}
	if err := requireReviewerRole(agentID); err != nil {
		return nil, err
	}

	result, err := ops.MergeWorktree(s.projectRoot, taskID, agentID)
	if err != nil {
		return nil, fmt.Errorf("wt-merge failed: %w", err)
	}

	return textResult(appendWarnings(
		fmt.Sprintf("Task %s merged to integration branch (commit: %s)", result.TaskID, result.MergeCommit),
		result.Warnings,
	))
}

// handleAnalyze implements the liza_analyze tool
// Maps to: liza analyze
func (s *Server) handleAnalyze(params map[string]any) (any, error) {
	result, err := ops.Analyze(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("analyze failed: %w", err)
	}

	if !result.Triggered {
		return textResult("Circuit breaker: OK — no patterns detected")
	}

	return textResult(fmt.Sprintf("Circuit breaker TRIGGERED: pattern=%s, severity=%s, report=%s",
		result.Pattern, result.Severity, result.ReportPath))
}

// handleSprintCheckpoint implements the liza_sprint_checkpoint tool
// Maps to: liza checkpoint
func (s *Server) handleSprintCheckpoint(params map[string]any) (any, error) {
	result, err := ops.SprintCheckpoint(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("checkpoint failed: %w", err)
	}

	return textResult(fmt.Sprintf("Sprint checkpoint created at %s. Report: %s. Agents will pause at their next check.",
		result.CheckpointAt.Format("2006-01-02T15:04:05Z07:00"), result.ReportPath))
}

// handleUpdateSprintMetrics implements the liza_update_sprint_metrics tool
// Maps to: liza update-sprint-metrics
func (s *Server) handleUpdateSprintMetrics(params map[string]any) (any, error) {
	metrics, err := ops.UpdateSprintMetrics(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("update sprint metrics failed: %w", err)
	}

	warnings := ops.CheckSuspiciousRates(metrics)

	var b strings.Builder
	fmt.Fprintf(&b, "Sprint metrics updated: %d done, %d in progress, %d blocked",
		metrics.TasksDone, metrics.TasksInProgress, metrics.TasksBlocked)
	for _, w := range warnings {
		b.WriteString("\n")
		b.WriteString(w)
	}

	return textResult(b.String())
}

// handleClearStaleReviews implements the liza_clear_stale_review_claims tool
// Maps to: liza clear-stale-review-claims
func (s *Server) handleClearStaleReviews(params map[string]any) (any, error) {
	count, err := ops.ClearStaleReviewClaims(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("clear stale review claims failed: %w", err)
	}

	return textResult(fmt.Sprintf("Cleared %d stale review claims", count))
}

// handleWriteCheckpoint implements the liza_write_checkpoint tool
func (s *Server) handleWriteCheckpoint(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}
	if err := requireDoerRole(agentID); err != nil {
		return nil, err
	}

	intent, err := requireString(params, "intent")
	if err != nil {
		return nil, err
	}

	validationPlan, err := requireString(params, "validation_plan")
	if err != nil {
		return nil, err
	}

	filesToModify := extractStringSlice(params, "files_to_modify")
	if len(filesToModify) == 0 {
		return nil, fmt.Errorf("files_to_modify parameter required (at least one file)")
	}

	assumptions := extractStringSlice(params, "assumptions")

	risks, _ := params["risks"].(string)
	tddNotRequired, _ := params["tdd_not_required"].(string)
	scopeExtensions := extractScopeExtensions(params, "scope_extensions")

	input := &ops.WriteCheckpointInput{
		TaskID:          taskID,
		AgentID:         agentID,
		Intent:          intent,
		ValidationPlan:  validationPlan,
		FilesToModify:   filesToModify,
		Assumptions:     assumptions,
		Risks:           risks,
		TDDNotRequired:  tddNotRequired,
		ScopeExtensions: scopeExtensions,
	}

	if err := ops.WriteCheckpoint(s.projectRoot, input); err != nil {
		return nil, fmt.Errorf("write checkpoint failed: %w", err)
	}

	return textResult(fmt.Sprintf("Pre-execution checkpoint written for task %s", taskID))
}

// handleSetTaskOutput implements the liza_set_task_output tool
func (s *Server) handleSetTaskOutput(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}
	if err := requireDoerRole(agentID); err != nil {
		return nil, err
	}

	rawOutput, _ := params["output"].([]any)
	if len(rawOutput) == 0 {
		return nil, fmt.Errorf("output parameter required (array of {desc, done_when, scope, spec_ref})")
	}
	output, err := extractOutputEntries(rawOutput)
	if err != nil {
		return nil, err
	}

	input := &ops.SetTaskOutputInput{
		TaskID:  taskID,
		AgentID: agentID,
		Output:  output,
	}

	if err := ops.SetTaskOutput(s.projectRoot, input); err != nil {
		return nil, fmt.Errorf("set task output failed: %w", err)
	}

	return textResult(fmt.Sprintf("Output set on task %s (%d entries)", taskID, len(output)))
}
