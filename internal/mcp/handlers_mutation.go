package mcp

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// handleAddTasks implements the liza_add_tasks tool (batch endpoint).
func (s *Server) handleAddTasks(params map[string]any) (any, error) {
	agentID, _ := params["agent_id"].(string)
	if agentID == "" {
		agentID = "orchestrator-1"
	}

	if err := requireRole(agentID, roles.RuntimeOrchestrator); err != nil {
		return nil, err
	}

	rawTasks, ok := params["tasks"].([]any)
	if !ok {
		return nil, fmt.Errorf("tasks parameter must be an array")
	}
	if len(rawTasks) == 0 {
		return nil, fmt.Errorf("tasks array must not be empty")
	}

	tasks, err := extractTaskInputs(rawTasks)
	if err != nil {
		return nil, err
	}

	input := &ops.AddTasksInput{Tasks: tasks, OrchestratorID: agentID}
	statePath := paths.New(s.projectRoot).StatePath()
	result, err := ops.AddTasks(statePath, s.logPath, input)
	if err != nil {
		return nil, fmt.Errorf("add tasks failed: %w", err)
	}

	return textResult(formatAddTasksResult(result))
}

// handleAddTaskCompat is a deprecated compatibility wrapper for liza_add_task.
// It wraps the single-task params into a batch call to handleAddTasks.
func (s *Server) handleAddTaskCompat(params map[string]any) (any, error) {
	agentID, _ := params["agent_id"].(string)
	task := make(map[string]any, len(params))
	for k, v := range params {
		if k != "agent_id" {
			task[k] = v
		}
	}
	return s.handleAddTasks(map[string]any{
		"tasks":    []any{task},
		"agent_id": agentID,
	})
}

// handleClaimTask implements the liza_claim_task tool
// Maps to: liza claim-task
func (s *Server) handleClaimTask(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	result, err := ops.ClaimTask(s.projectRoot, taskID, agentID)
	if err != nil {
		return nil, fmt.Errorf("claim task failed: %w", err)
	}

	return textResult(appendWarnings(
		fmt.Sprintf("Task %s claimed by %s (from %s), worktree: %s",
			result.TaskID, result.AgentID, result.SourceStatus, result.WorktreeRel),
		result.Warnings))
}

// handleSubmitForReview implements the liza_submit_for_review tool
// Maps to: liza submit-for-review
func (s *Server) handleSubmitForReview(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	commitSHA, err := requireString(params, "commit_sha")
	if err != nil {
		return nil, err
	}

	agentID, err := requireString(params, "agent_id")
	if err != nil {
		return nil, err
	}

	result, err := ops.SubmitForReview(s.projectRoot, taskID, commitSHA, agentID)
	if err != nil {
		return nil, fmt.Errorf("submit for review failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s submitted for review (commit: %s)", result.TaskID, result.ReviewCommit))
}

// handleHandoff implements the liza_handoff tool
// Maps to: liza handoff
func (s *Server) handleHandoff(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	summary, err := requireString(params, "summary")
	if err != nil {
		return nil, err
	}

	nextAction, err := requireString(params, "next_action")
	if err != nil {
		return nil, err
	}

	result, err := ops.Handoff(s.projectRoot, taskID, summary, nextAction, agentID)
	if err != nil {
		return nil, fmt.Errorf("handoff failed: %w", err)
	}

	return textResult(fmt.Sprintf("Handoff initiated for task %s by %s", result.TaskID, result.AgentID))
}

// handleSubmitVerdict implements the liza_submit_verdict tool
// Maps to: liza submit-verdict
func (s *Server) handleSubmitVerdict(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	verdict, err := requireString(params, "verdict")
	if err != nil {
		return nil, err
	}

	agentID, err := requireString(params, "agent_id")
	if err != nil {
		return nil, err
	}

	reason, _ := params["reason"].(string)

	result, err := ops.SubmitVerdict(s.projectRoot, taskID, verdict, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("submit verdict failed: %w", err)
	}

	return textResult(fmt.Sprintf("Verdict %s submitted for task %s", result.Verdict, result.TaskID))
}

// handleMarkBlocked implements the liza_mark_blocked tool
// Maps to: liza mark-blocked
func (s *Server) handleMarkBlocked(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	reason, err := requireString(params, "reason")
	if err != nil {
		return nil, err
	}

	questions := extractStringSlice(params, "questions")
	if len(questions) == 0 {
		return nil, fmt.Errorf("questions parameter required (1-3 questions)")
	}

	_, err = ops.MarkBlocked(s.projectRoot, taskID, reason, questions, agentID)
	if err != nil {
		return nil, fmt.Errorf("mark blocked failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s marked as BLOCKED", taskID))
}

// handleReleaseClaim implements the liza_release_claim tool
// Maps to: liza release-claim
func (s *Server) handleReleaseClaim(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	role, err := requireString(params, "role")
	if err != nil {
		return nil, err
	}

	agentID, err := requireString(params, "agent_id")
	if err != nil {
		return nil, err
	}

	if err := authorizeClaimRelease(agentID, role); err != nil {
		return nil, err
	}

	reason, _ := params["reason"].(string)
	force, _ := params["force"].(bool)

	result, err := ops.ReleaseClaim(s.projectRoot, taskID, role, force, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("release claim failed: %w", err)
	}

	var msg string
	switch {
	case result.ReleasedReviewer && result.ReleasedDoer:
		msg = "Released reviewer and doer claims for task " + result.TaskID
	case result.ReleasedReviewer:
		msg = "Released reviewer claim for task " + result.TaskID
	case result.ReleasedDoer:
		msg = "Released doer claim for task " + result.TaskID
	default:
		msg = "Claim released for task " + result.TaskID
	}
	return textResult(msg)
}

// handleSupersede implements the liza_supersede_task tool
// Maps to: liza supersede-task
func (s *Server) handleSupersede(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	reason, err := requireString(params, "reason")
	if err != nil {
		return nil, err
	}

	replacementIDs := extractStringSlice(params, "replacement_ids")

	result, err := ops.SupersedeTask(s.projectRoot, taskID, replacementIDs, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("supersede task failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s superseded (was %s)", result.TaskID, result.OriginalStatus))
}

// handleDeleteAgent implements the liza_delete_agent tool
// Maps to: liza delete agent
func (s *Server) handleDeleteAgent(params map[string]any) (any, error) {
	targetAgentID, err := requireString(params, "target_agent_id")
	if err != nil {
		return nil, err
	}

	reason, err := requireString(params, "reason")
	if err != nil {
		return nil, err
	}

	force, _ := params["force"].(bool)

	result, err := ops.DeleteAgent(s.projectRoot, targetAgentID, force, false, reason)
	if err != nil {
		return nil, fmt.Errorf("delete agent failed: %w", err)
	}

	return textResult(fmt.Sprintf("Agent %s deleted", result.AgentID))
}
