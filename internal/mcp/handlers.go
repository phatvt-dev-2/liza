package mcp

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/paths"
)

// Version and BuildCommit are set from the embedded package's build-time
// variables when the MCP server binary starts. Defaults for dev/test use.
var (
	Version     = "dev"
	BuildCommit = "unknown"
)

// textResult builds a standard MCP text content response.
func textResult(msg string) (any, error) {
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": msg,
			},
		},
	}, nil
}

// resourceContent builds a standard MCP resource content response.
func resourceContent(uri, mimeType, text string) any {
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      uri,
				"mimeType": mimeType,
				"text":     text,
			},
		},
	}
}

// inspectResource reads a Liza resource via the inspect command.
func (s *Server) inspectResource(uri string, args ...string) (any, error) {
	opts := commands.InspectOptions{
		Format:      "json",
		ProjectRoot: s.projectRoot,
		Internal:    false,
	}
	result, err := commands.InspectCommand(args, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", uri, err)
	}
	return resourceContent(uri, "application/json", result), nil
}

// requireString extracts a required non-empty string parameter.
func requireString(params map[string]any, key string) (string, error) {
	v, ok := params[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("%s parameter required", key)
	}
	return v, nil
}

// requireTaskAndAgent extracts the common task_id + agent_id pair.
func requireTaskAndAgent(params map[string]any) (taskID, agentID string, err error) {
	taskID, err = requireString(params, "task_id")
	if err != nil {
		return "", "", err
	}
	agentID, err = requireString(params, "agent_id")
	if err != nil {
		return "", "", err
	}
	return taskID, agentID, nil
}

// handleGet implements the liza_get tool
// Maps to: liza get <query>
func (s *Server) handleGet(params map[string]any) (any, error) {
	query, err := requireString(params, "query")
	if err != nil {
		return nil, err
	}

	format := "json"
	if f, ok := params["format"].(string); ok && f != "" {
		format = f
	}

	opts := commands.InspectOptions{
		Format:      format,
		ProjectRoot: s.projectRoot,
		Internal:    false, // Get formatted output
	}

	result, err := commands.InspectCommand([]string{query}, opts)
	if err != nil {
		return nil, fmt.Errorf("inspect command failed: %w", err)
	}

	return textResult(result)
}

// handleStatus implements the liza_status tool
// Maps to: liza status
func (s *Server) handleStatus(params map[string]any) (any, error) {
	opts := commands.StatusOptions{
		ProjectRoot: s.projectRoot,
	}

	result, err := commands.StatusCommand(opts)
	if err != nil {
		return nil, fmt.Errorf("status command failed: %w", err)
	}

	return textResult(result)
}

// handleValidate implements the liza_validate tool
// Maps to: liza validate
func (s *Server) handleValidate(params map[string]any) (any, error) {
	statePath := paths.New(s.projectRoot).StatePath()

	skipSpecFileCheck := false
	if skip, ok := params["skip_spec_check"].(bool); ok {
		skipSpecFileCheck = skip
	}

	err := commands.ValidateCommand(statePath, skipSpecFileCheck)

	var resultText string
	if err != nil {
		resultText = fmt.Sprintf("Validation failed: %v", err)
	} else {
		resultText = "Validation passed: workspace state is consistent"
	}

	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": resultText,
			},
		},
		"isError": err != nil,
	}, nil
}

// handleVersion implements the liza_version tool
// Maps to: liza version
func (s *Server) handleVersion(params map[string]any) (any, error) {
	return textResult(fmt.Sprintf("liza-mcp version %s (commit: %s)", Version, BuildCommit))
}

// handleResourceReadInternal reads a resource by URI
func (s *Server) handleResourceReadInternal(uri string) (any, error) {
	switch uri {
	case "liza://state":
		return s.readStateResource()
	case "liza://tasks":
		return s.inspectResource(uri, "tasks")
	case "liza://agents":
		return s.inspectResource(uri, "agents")
	default:
		const taskPrefix = "liza://tasks/"
		if len(uri) > len(taskPrefix) && uri[:len(taskPrefix)] == taskPrefix {
			taskID := uri[len(taskPrefix):]
			return s.inspectResource(uri, "tasks", taskID)
		}
		return nil, fmt.Errorf("unknown resource URI: %s", uri)
	}
}

// readStateResource returns the raw state.yaml content
func (s *Server) readStateResource() (any, error) {
	statePath := paths.New(s.projectRoot).StatePath()

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return resourceContent("liza://state", "application/x-yaml", string(data)), nil
}

// handleAddTask implements the liza_add_task tool
// Maps to: liza add-task
func (s *Server) handleAddTask(params map[string]any) (any, error) {
	id, err := requireString(params, "id")
	if err != nil {
		return nil, err
	}

	description, err := requireString(params, "desc")
	if err != nil {
		return nil, err
	}

	specRef, err := requireString(params, "spec")
	if err != nil {
		return nil, err
	}

	doneWhen, err := requireString(params, "done")
	if err != nil {
		return nil, err
	}

	scope, err := requireString(params, "scope")
	if err != nil {
		return nil, err
	}

	agentID, _ := params["agent_id"].(string)
	if agentID == "" {
		agentID = "planner-1"
	}

	priority := 1
	if p, ok := params["priority"].(float64); ok {
		priority = int(p)
	} else if p, ok := params["priority"].(int); ok {
		priority = p
	}

	var dependsOn []string
	if deps, ok := params["depends"].([]any); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				dependsOn = append(dependsOn, depStr)
			}
		}
	}

	taskType, _ := params["type"].(string)

	input := &commands.TaskInput{
		ID:          id,
		Type:        taskType,
		Description: description,
		SpecRef:     specRef,
		DoneWhen:    doneWhen,
		Scope:       scope,
		Priority:    priority,
		DependsOn:   dependsOn,
	}

	statePath := paths.New(s.projectRoot).StatePath()
	if err := commands.AddTaskCommand(statePath, s.logPath, input, agentID); err != nil {
		return nil, fmt.Errorf("add task failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s added successfully", id))
}

// handleClaimTask implements the liza_claim_task tool
// Maps to: liza claim-task
func (s *Server) handleClaimTask(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	if err := commands.ClaimTaskCommand(s.projectRoot, taskID, agentID); err != nil {
		return nil, fmt.Errorf("claim task failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s claimed by %s", taskID, agentID))
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

	if err := commands.SubmitForReviewCommand(s.projectRoot, taskID, commitSHA, agentID); err != nil {
		return nil, fmt.Errorf("submit for review failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s submitted for review", taskID))
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

	if err := commands.HandoffCommand(s.projectRoot, taskID, summary, nextAction, agentID); err != nil {
		return nil, fmt.Errorf("handoff failed: %w", err)
	}

	return textResult(fmt.Sprintf("Handoff initiated for task %s", taskID))
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

	if err := commands.SubmitVerdictCommand(s.projectRoot, taskID, verdict, reason, agentID); err != nil {
		return nil, fmt.Errorf("submit verdict failed: %w", err)
	}

	return textResult(fmt.Sprintf("Verdict %s submitted for task %s", verdict, taskID))
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

	var questions []string
	if questionsRaw, ok := params["questions"].([]any); ok {
		for _, q := range questionsRaw {
			if qStr, ok := q.(string); ok {
				questions = append(questions, qStr)
			}
		}
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("questions parameter required (1-3 questions)")
	}

	if err := commands.MarkBlockedCommand(s.projectRoot, taskID, reason, questions, agentID); err != nil {
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

	reason, _ := params["reason"].(string)
	force, _ := params["force"].(bool)

	if err := commands.ReleaseClaimCommand(s.projectRoot, taskID, role, force, reason, agentID); err != nil {
		return nil, fmt.Errorf("release claim failed: %w", err)
	}

	return textResult(fmt.Sprintf("Claim released for task %s", taskID))
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

	var replacementIDs []string
	if ids, ok := params["replacement_ids"].([]any); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				replacementIDs = append(replacementIDs, idStr)
			}
		}
	}

	if err := commands.SupersedeTaskCommand(s.projectRoot, taskID, replacementIDs, reason, agentID); err != nil {
		return nil, fmt.Errorf("supersede task failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s superseded", taskID))
}

// handleWtCreate implements the liza_wt_create tool
// Maps to: liza wt-create
func (s *Server) handleWtCreate(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	fresh, _ := params["fresh"].(bool)

	if err := commands.WtCreateCommand(s.projectRoot, taskID, fresh); err != nil {
		return nil, fmt.Errorf("wt-create failed: %w", err)
	}

	return textResult(fmt.Sprintf("Worktree created for task %s", taskID))
}

// handleWtDelete implements the liza_wt_delete tool
// Maps to: liza wt-delete
func (s *Server) handleWtDelete(params map[string]any) (any, error) {
	taskID, err := requireString(params, "task_id")
	if err != nil {
		return nil, err
	}

	if err := commands.WtDeleteCommand(s.projectRoot, taskID); err != nil {
		return nil, fmt.Errorf("wt-delete failed: %w", err)
	}

	return textResult(fmt.Sprintf("Worktree deleted for task %s", taskID))
}

// handleWtMerge implements the liza_wt_merge tool
// Maps to: liza wt-merge
func (s *Server) handleWtMerge(params map[string]any) (any, error) {
	taskID, agentID, err := requireTaskAndAgent(params)
	if err != nil {
		return nil, err
	}

	if err := commands.WtMergeCommand(s.projectRoot, taskID, agentID); err != nil {
		return nil, fmt.Errorf("wt-merge failed: %w", err)
	}

	return textResult(fmt.Sprintf("Task %s merged to integration branch", taskID))
}

// handleAnalyze implements the liza_analyze tool
// Maps to: liza analyze
func (s *Server) handleAnalyze(params map[string]any) (any, error) {
	if err := commands.AnalyzeCommand(s.projectRoot); err != nil {
		return nil, fmt.Errorf("analyze failed: %w", err)
	}

	return textResult("Circuit breaker analysis complete")
}

// handleCheckpoint implements the liza_checkpoint tool
// Maps to: liza checkpoint
func (s *Server) handleCheckpoint(params map[string]any) (any, error) {
	if err := commands.CheckpointCommand(s.projectRoot); err != nil {
		return nil, fmt.Errorf("checkpoint failed: %w", err)
	}

	return textResult("Sprint checkpoint created. Agents will pause at their next check.")
}

// handleUpdateSprintMetrics implements the liza_update_sprint_metrics tool
// Maps to: liza update-sprint-metrics
func (s *Server) handleUpdateSprintMetrics(params map[string]any) (any, error) {
	if err := commands.UpdateSprintMetricsCommand(s.projectRoot); err != nil {
		return nil, fmt.Errorf("update sprint metrics failed: %w", err)
	}

	return textResult("Sprint metrics updated successfully")
}

// handleClearStaleReviews implements the liza_clear_stale_review_claims tool
// Maps to: liza clear-stale-review-claims
func (s *Server) handleClearStaleReviews(params map[string]any) (any, error) {
	count, err := commands.ClearStaleReviewClaimsCommand(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("clear stale review claims failed: %w", err)
	}

	return textResult(fmt.Sprintf("Cleared %d stale review claims", count))
}

// handleDeleteAgent implements the liza_delete_agent tool
// Maps to: liza delete agent
func (s *Server) handleDeleteAgent(params map[string]any) (any, error) {
	agentID, err := requireString(params, "agent_id")
	if err != nil {
		return nil, err
	}

	reason, err := requireString(params, "reason")
	if err != nil {
		return nil, err
	}

	force, _ := params["force"].(bool)

	if err := commands.DeleteAgentCommand(s.projectRoot, agentID, force, reason); err != nil {
		return nil, fmt.Errorf("delete agent failed: %w", err)
	}

	return textResult(fmt.Sprintf("Agent %s deleted", agentID))
}
