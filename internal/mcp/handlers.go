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

// handleGet implements the liza_get tool
// Maps to: liza get <query>
func (s *Server) handleGet(params map[string]any) (any, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter required")
	}

	// Get format (default: json)
	format := "json"
	if f, ok := params["format"].(string); ok && f != "" {
		format = f
	}

	// Build inspect options
	opts := commands.InspectOptions{
		Format:      format,
		ProjectRoot: s.projectRoot,
		Internal:    false, // Get formatted output
	}

	// Call existing inspect command
	result, err := commands.InspectCommand([]string{query}, opts)
	if err != nil {
		return nil, fmt.Errorf("inspect command failed: %w", err)
	}

	// Return MCP-formatted response
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": result,
			},
		},
	}, nil
}

// handleStatus implements the liza_status tool
// Maps to: liza status
func (s *Server) handleStatus(params map[string]any) (any, error) {
	// Build status options
	opts := commands.StatusOptions{
		ProjectRoot: s.projectRoot,
	}

	// Call existing status command
	result, err := commands.StatusCommand(opts)
	if err != nil {
		return nil, fmt.Errorf("status command failed: %w", err)
	}

	// Return MCP-formatted response
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": result,
			},
		},
	}, nil
}

// handleValidate implements the liza_validate tool
// Maps to: liza validate
func (s *Server) handleValidate(params map[string]any) (any, error) {
	statePath := paths.New(s.projectRoot).StatePath()

	// Get skipSpecFileCheck parameter (default: false)
	skipSpecFileCheck := false
	if skip, ok := params["skip_spec_file_check"].(bool); ok {
		skipSpecFileCheck = skip
	}

	// Call existing validate command
	err := commands.ValidateCommand(statePath, skipSpecFileCheck)

	var resultText string
	if err != nil {
		resultText = fmt.Sprintf("Validation failed: %v", err)
	} else {
		resultText = "Validation passed: workspace state is consistent"
	}

	// Return MCP-formatted response
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
	versionInfo := fmt.Sprintf("liza-mcp version %s (commit: %s)", Version, BuildCommit)

	// Return MCP-formatted response
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": versionInfo,
			},
		},
	}, nil
}

// handleResourceReadInternal reads a resource by URI
func (s *Server) handleResourceReadInternal(uri string) (any, error) {
	switch uri {
	case "liza://state":
		return s.readStateResource()
	case "liza://tasks":
		return s.readTasksResource()
	case "liza://agents":
		return s.readAgentsResource()
	default:
		// Check for specific task resource: liza://tasks/{id}
		const taskPrefix = "liza://tasks/"
		if len(uri) > len(taskPrefix) && uri[:len(taskPrefix)] == taskPrefix {
			taskID := uri[len(taskPrefix):]
			return s.readSpecificTaskResource(taskID)
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

	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      "liza://state",
				"mimeType": "application/x-yaml",
				"text":     string(data),
			},
		},
	}, nil
}

// readTasksResource returns all tasks as JSON
func (s *Server) readTasksResource() (any, error) {
	opts := commands.InspectOptions{
		Format:      "json",
		ProjectRoot: s.projectRoot,
		Internal:    false,
	}

	result, err := commands.InspectCommand([]string{"tasks"}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks: %w", err)
	}

	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      "liza://tasks",
				"mimeType": "application/json",
				"text":     result,
			},
		},
	}, nil
}

// readSpecificTaskResource returns a specific task as JSON
func (s *Server) readSpecificTaskResource(taskID string) (any, error) {
	opts := commands.InspectOptions{
		Format:      "json",
		ProjectRoot: s.projectRoot,
		Internal:    false,
	}

	result, err := commands.InspectCommand([]string{"tasks", taskID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read task %s: %w", taskID, err)
	}

	uri := fmt.Sprintf("liza://tasks/%s", taskID)
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      uri,
				"mimeType": "application/json",
				"text":     result,
			},
		},
	}, nil
}

// readAgentsResource returns all agents as JSON
func (s *Server) readAgentsResource() (any, error) {
	opts := commands.InspectOptions{
		Format:      "json",
		ProjectRoot: s.projectRoot,
		Internal:    false,
	}

	result, err := commands.InspectCommand([]string{"agents"}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read agents: %w", err)
	}

	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      "liza://agents",
				"mimeType": "application/json",
				"text":     result,
			},
		},
	}, nil
}

// ============================================================================
// Phase 2: Mutation Tool Handlers
// ============================================================================

// handleAddTask implements the liza_add_task tool
// Maps to: liza add-task
func (s *Server) handleAddTask(params map[string]any) (any, error) {
	// Extract and validate parameters
	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id parameter required")
	}

	description, ok := params["description"].(string)
	if !ok || description == "" {
		return nil, fmt.Errorf("description parameter required")
	}

	specRef, ok := params["spec_ref"].(string)
	if !ok || specRef == "" {
		return nil, fmt.Errorf("spec_ref parameter required")
	}

	doneWhen, ok := params["done_when"].(string)
	if !ok || doneWhen == "" {
		return nil, fmt.Errorf("done_when parameter required")
	}

	scope, ok := params["scope"].(string)
	if !ok || scope == "" {
		return nil, fmt.Errorf("scope parameter required")
	}

	agentID, _ := params["agent_id"].(string)
	if agentID == "" {
		agentID = "planner-1"
	}

	// Optional parameters
	priority := 1
	if p, ok := params["priority"].(float64); ok {
		priority = int(p)
	} else if p, ok := params["priority"].(int); ok {
		priority = p
	}

	var dependsOn []string
	if deps, ok := params["depends_on"].([]any); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				dependsOn = append(dependsOn, depStr)
			}
		}
	}

	// Build task input
	input := &commands.TaskInput{
		ID:          id,
		Description: description,
		SpecRef:     specRef,
		DoneWhen:    doneWhen,
		Scope:       scope,
		Priority:    priority,
		DependsOn:   dependsOn,
	}

	// Call existing command
	statePath := paths.New(s.projectRoot).StatePath()
	err := commands.AddTaskCommand(statePath, s.logPath, input, agentID)
	if err != nil {
		return nil, fmt.Errorf("add task failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s added successfully", id),
			},
		},
	}, nil
}

// handleClaimTask implements the liza_claim_task tool
// Maps to: liza claim-task
func (s *Server) handleClaimTask(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	// Call existing command (uses three-phase commit pattern)
	err := commands.ClaimTaskCommand(s.projectRoot, taskID, agentID)
	if err != nil {
		return nil, fmt.Errorf("claim task failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s claimed by %s", taskID, agentID),
			},
		},
	}, nil
}

// handleSubmitForReview implements the liza_submit_for_review tool
// Maps to: liza submit-for-review
func (s *Server) handleSubmitForReview(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	commitSHA, ok := params["commit_sha"].(string)
	if !ok || commitSHA == "" {
		return nil, fmt.Errorf("commit_sha parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	// Call existing command
	err := commands.SubmitForReviewCommand(s.projectRoot, taskID, commitSHA, agentID)
	if err != nil {
		return nil, fmt.Errorf("submit for review failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s submitted for review", taskID),
			},
		},
	}, nil
}

// handleSubmitVerdict implements the liza_submit_verdict tool
// Maps to: liza submit-verdict
func (s *Server) handleSubmitVerdict(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	verdict, ok := params["verdict"].(string)
	if !ok || verdict == "" {
		return nil, fmt.Errorf("verdict parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	// Optional reason
	reason, _ := params["reason"].(string)

	// Call existing command
	err := commands.SubmitVerdictCommand(s.projectRoot, taskID, verdict, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("submit verdict failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Verdict %s submitted for task %s", verdict, taskID),
			},
		},
	}, nil
}

// handleMarkBlocked implements the liza_mark_blocked tool
// Maps to: liza mark-blocked
func (s *Server) handleMarkBlocked(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	reason, ok := params["reason"].(string)
	if !ok || reason == "" {
		return nil, fmt.Errorf("reason parameter required")
	}

	// Extract questions array
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

	// Call existing command
	err := commands.MarkBlockedCommand(s.projectRoot, taskID, reason, questions, agentID)
	if err != nil {
		return nil, fmt.Errorf("mark blocked failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s marked as BLOCKED", taskID),
			},
		},
	}, nil
}

// handleReleaseClaim implements the liza_release_claim tool
// Maps to: liza release-claim
func (s *Server) handleReleaseClaim(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	role, ok := params["role"].(string)
	if !ok || role == "" {
		return nil, fmt.Errorf("role parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	// Optional parameters
	reason, _ := params["reason"].(string)
	force, _ := params["force"].(bool)

	// Call existing command
	err := commands.ReleaseClaimCommand(s.projectRoot, taskID, role, force, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("release claim failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Claim released for task %s", taskID),
			},
		},
	}, nil
}

// handleSupersede implements the liza_supersede_task tool
// Maps to: liza supersede-task
func (s *Server) handleSupersede(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	reason, ok := params["reason"].(string)
	if !ok || reason == "" {
		return nil, fmt.Errorf("reason parameter required")
	}

	// Extract replacement IDs
	var replacementIDs []string
	if ids, ok := params["replacement_ids"].([]any); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				replacementIDs = append(replacementIDs, idStr)
			}
		}
	}

	// Call existing command
	err := commands.SupersedeTaskCommand(s.projectRoot, taskID, replacementIDs, reason, agentID)
	if err != nil {
		return nil, fmt.Errorf("supersede task failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s superseded", taskID),
			},
		},
	}, nil
}

// ============================================================================
// Phase 3: Worktree Operation Handlers
// ============================================================================

// handleWtCreate implements the liza_wt_create tool
// Maps to: liza wt-create
func (s *Server) handleWtCreate(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	// Optional fresh flag
	fresh, _ := params["fresh"].(bool)

	// Call existing command
	err := commands.WtCreateCommand(s.projectRoot, taskID, fresh)
	if err != nil {
		return nil, fmt.Errorf("wt-create failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Worktree created for task %s", taskID),
			},
		},
	}, nil
}

// handleWtDelete implements the liza_wt_delete tool
// Maps to: liza wt-delete
func (s *Server) handleWtDelete(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	// Call existing command
	err := commands.WtDeleteCommand(s.projectRoot, taskID)
	if err != nil {
		return nil, fmt.Errorf("wt-delete failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Worktree deleted for task %s", taskID),
			},
		},
	}, nil
}

// handleWtMerge implements the liza_wt_merge tool
// Maps to: liza wt-merge
func (s *Server) handleWtMerge(params map[string]any) (any, error) {
	taskID, ok := params["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id parameter required")
	}

	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	// Call existing command
	err := commands.WtMergeCommand(s.projectRoot, taskID, agentID)
	if err != nil {
		return nil, fmt.Errorf("wt-merge failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Task %s merged to integration branch", taskID),
			},
		},
	}, nil
}

// ============================================================================
// Phase 3: Analysis & Utility Handlers
// ============================================================================

// handleAnalyze implements the liza_analyze tool
// Maps to: liza analyze
func (s *Server) handleAnalyze(params map[string]any) (any, error) {
	// Call existing command
	err := commands.AnalyzeCommand(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("analyze failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": "Circuit breaker analysis complete",
			},
		},
	}, nil
}

// handleUpdateSprintMetrics implements the liza_update_sprint_metrics tool
// Maps to: liza update-sprint-metrics
func (s *Server) handleUpdateSprintMetrics(params map[string]any) (any, error) {
	// Call existing command
	err := commands.UpdateSprintMetricsCommand(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("update sprint metrics failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": "Sprint metrics updated successfully",
			},
		},
	}, nil
}

// handleClearStaleReviews implements the liza_clear_stale_review_claims tool
// Maps to: liza clear-stale-review-claims
func (s *Server) handleClearStaleReviews(params map[string]any) (any, error) {
	// Call existing command
	count, err := commands.ClearStaleReviewClaimsCommand(s.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("clear stale review claims failed: %w", err)
	}

	// Return success message with count
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Cleared %d stale review claims", count),
			},
		},
	}, nil
}

// handleDeleteAgent implements the liza_delete_agent tool
// Maps to: liza delete agent
func (s *Server) handleDeleteAgent(params map[string]any) (any, error) {
	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, fmt.Errorf("agent_id parameter required")
	}

	reason, ok := params["reason"].(string)
	if !ok || reason == "" {
		return nil, fmt.Errorf("reason parameter required")
	}

	// Optional force flag
	force, _ := params["force"].(bool)

	// Call existing command
	err := commands.DeleteAgentCommand(s.projectRoot, agentID, force, reason)
	if err != nil {
		return nil, fmt.Errorf("delete agent failed: %w", err)
	}

	// Return success message
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf("Agent %s deleted", agentID),
			},
		},
	}, nil
}
