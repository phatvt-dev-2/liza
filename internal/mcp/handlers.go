package mcp

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
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

// extractStringSlice extracts an optional []string from a JSON params map.
// JSON arrays arrive as []any; non-string elements are silently skipped.
func extractStringSlice(params map[string]any, key string) []string {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// extractScopeExtensions extracts an optional []ScopeExtensionEntry from a JSON params map.
// JSON arrays arrive as []any of map[string]any; malformed entries are silently skipped.
func extractScopeExtensions(params map[string]any, key string) []ops.ScopeExtensionEntry {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]ops.ScopeExtensionEntry, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		file, _ := m["file"].(string)
		justification, _ := m["justification"].(string)
		if file != "" && justification != "" {
			out = append(out, ops.ScopeExtensionEntry{
				File:          file,
				Justification: justification,
			})
		}
	}
	return out
}

// appendWarnings appends warning lines to a message string.
func appendWarnings(msg string, warnings []string) string {
	if len(warnings) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	for _, w := range warnings {
		b.WriteString("\nWarning: ")
		b.WriteString(w)
	}
	return b.String()
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

// RoleError indicates an agent does not have the required role for an operation.
// The message is intentionally client-facing so agents receive actionable feedback.
type RoleError struct {
	Expected []string
	Got      string
	AgentID  string
}

func (e *RoleError) Error() string {
	return fmt.Sprintf("requires one of %v roles (got %s from %s)", e.Expected, e.Got, e.AgentID)
}

// requireRole validates agent ID format and that it matches the expected runtime role.
func requireRole(agentID, expectedRole string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID) // cannot fail after ValidateFormat
	if role != expectedRole {
		return &RoleError{Expected: []string{expectedRole}, Got: role, AgentID: agentID}
	}
	return nil
}

// requireDoerRole validates agent ID format and that it has a doer role.
func requireDoerRole(agentID string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID)
	if !roles.IsDoerRole(role) {
		return &RoleError{Expected: roles.DoerRoles(), Got: role, AgentID: agentID}
	}
	return nil
}

// requireReviewerRole validates agent ID format and that it has a reviewer role.
func requireReviewerRole(agentID string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID)
	if !roles.IsReviewerRole(role) {
		return &RoleError{Expected: roles.ReviewerRoles(), Got: role, AgentID: agentID}
	}
	return nil
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

	// Split on "/" so "tasks/<id>" becomes ["tasks", "<id>"]
	args := strings.SplitN(query, "/", 2)

	result, err := commands.InspectCommand(args, opts)
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
		if taskID, ok := strings.CutPrefix(uri, "liza://tasks/"); ok {
			return s.inspectResource(uri, "tasks", taskID)
		}
		return nil, fmt.Errorf("unknown resource URI: %s", uri)
	}
}

// readStateResource returns the raw state.yaml content under flock protection.
func (s *Server) readStateResource() (any, error) {
	data, err := s.bb.ReadRaw()
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return resourceContent("liza://state", "application/x-yaml", string(data)), nil
}

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

// extractTaskInputs converts a raw JSON array into []ops.AddTaskInput.
// Returns indexed errors for malformed elements.
func extractTaskInputs(raw []any) ([]ops.AddTaskInput, error) {
	out := make([]ops.AddTaskInput, 0, len(raw))
	for i, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tasks[%d]: must be an object, got %T", i, v)
		}

		id := stringFromMap(m, "id")
		if id == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'id'", i)
		}
		desc := stringFromMap(m, "desc")
		if desc == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'desc'", i)
		}
		spec := stringFromMap(m, "spec")
		if spec == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'spec'", i)
		}
		done := stringFromMap(m, "done")
		if done == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'done'", i)
		}
		scope := stringFromMap(m, "scope")
		if scope == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'scope'", i)
		}

		priority := 1
		if p, ok := m["priority"].(float64); ok {
			priority = int(p)
		} else if p, ok := m["priority"].(int); ok {
			priority = p
		}

		depends := extractStringSlice(m, "depends")
		taskType := stringFromMap(m, "type")
		rolePair := stringFromMap(m, "role_pair")

		out = append(out, ops.AddTaskInput{
			ID:          id,
			Type:        taskType,
			RolePair:    rolePair,
			Description: desc,
			SpecRef:     spec,
			DoneWhen:    done,
			Scope:       scope,
			Priority:    priority,
			DependsOn:   depends,
		})
	}
	return out, nil
}

// formatAddTasksResult builds a human-readable summary of batch results.
func formatAddTasksResult(result *ops.AddTasksResult) string {
	succeeded := 0
	for _, r := range result.Results {
		if r.Success {
			succeeded++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Added %d/%d tasks", succeeded, len(result.Results))
	for _, r := range result.Results {
		if r.Success {
			fmt.Fprintf(&b, "\n  %s: added", r.TaskID)
			for _, w := range r.Warnings {
				fmt.Fprintf(&b, " (warning: %s)", w)
			}
		} else {
			fmt.Fprintf(&b, "\n  %s: error: %s", r.TaskID, r.Error)
		}
	}
	return b.String()
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
	if err := requireDoerRole(agentID); err != nil {
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

	if err := requireDoerRole(agentID); err != nil {
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
	if err := requireDoerRole(agentID); err != nil {
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

	if err := requireReviewerRole(agentID); err != nil {
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

// authorizeClaimRelease validates that the agent's runtime role is authorized
// to release the requested claim type. Orchestrator can release any claim;
// others can only release claims matching their own role category.
func authorizeClaimRelease(agentID, claimRole string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	agentRole, _ := identity.ExtractRole(agentID)
	switch agentRole {
	case roles.RuntimeOrchestrator:
		return nil
	case roles.RuntimeCoder, roles.RuntimeCodePlanner:
		if claimRole != "coder" {
			return fmt.Errorf("agent %s (role %s) can only release coder claims", agentID, agentRole)
		}
	case roles.RuntimeCodeReviewer, roles.RuntimeCodePlanReviewer:
		if claimRole != "code-reviewer" {
			return fmt.Errorf("agent %s (role %s) can only release code-reviewer claims", agentID, agentRole)
		}
	default:
		return fmt.Errorf("agent %s has unrecognized role %q for claim release", agentID, agentRole)
	}
	return nil
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
	case result.ReleasedReviewer && result.ReleasedCoder:
		msg = "Released reviewer and coder claims for task " + result.TaskID
	case result.ReleasedReviewer:
		msg = "Released reviewer claim for task " + result.TaskID
	case result.ReleasedCoder:
		msg = "Released coder claim for task " + result.TaskID
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
	if err := requireRole(agentID, roles.RuntimeOrchestrator); err != nil {
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

// extractOutputEntries converts a raw JSON array into []models.OutputEntry.
// Returns an error if any element is not an object (strict — no silent drops).
func extractOutputEntries(raw []any) ([]models.OutputEntry, error) {
	out := make([]models.OutputEntry, 0, len(raw))
	for i, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("output[%d] must be an object, got %T", i, v)
		}
		entry := models.OutputEntry{
			Desc:     stringFromMap(m, "desc"),
			DoneWhen: stringFromMap(m, "done_when"),
			Scope:    stringFromMap(m, "scope"),
			SpecRef:  stringFromMap(m, "spec_ref"),
		}
		out = append(out, entry)
	}
	return out, nil
}

// stringFromMap extracts a string value from a map, returning "" if absent or wrong type.
func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
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

	result, err := ops.DeleteAgent(s.projectRoot, agentID, force, false, reason)
	if err != nil {
		return nil, fmt.Errorf("delete agent failed: %w", err)
	}

	return textResult(fmt.Sprintf("Agent %s deleted", result.AgentID))
}
