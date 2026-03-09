package mcp

import (
	"github.com/liza-mas/liza/internal/mcp/protocol"
)

// registerTool registers a tool with its handler
func (s *Server) registerTool(tool protocol.Tool, handler ToolHandler) {
	s.tools[tool.Name] = tool
	s.handlers[tool.Name] = handler
}

// registerResource registers a resource
func (s *Server) registerResource(resource protocol.Resource) {
	s.resources[resource.URI] = resource
}

// registerReadOnlyTools registers Phase 1 read-only tools
func (s *Server) registerReadOnlyTools() {
	// liza_get tool
	s.registerTool(protocol.Tool{
		Name:        "liza_get",
		Description: "Query Liza state (tasks, agents, logs, etc.)",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"query": {
					Type:        "string",
					Description: "Query path (e.g., 'tasks', 'tasks/<id>', 'agents', 'agents/<id>')",
				},
				"format": {
					Type:        "string",
					Description: "Output format (json, yaml, text)",
					Default:     "json",
				},
			},
			Required: []string{"query"},
		},
	}, s.handleGet)

	// liza_status tool
	s.registerTool(protocol.Tool{
		Name:        "liza_status",
		Description: "Get current workspace status summary",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleStatus)

	// liza_validate tool
	s.registerTool(protocol.Tool{
		Name:        "liza_validate",
		Description: "Validate workspace state consistency",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"skip_spec_check": {
					Type:        "boolean",
					Description: "Skip validation of spec file existence",
					Default:     false,
				},
			},
		},
	}, s.handleValidate)

	// liza_version tool
	s.registerTool(protocol.Tool{
		Name:        "liza_version",
		Description: "Get Liza version information",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleVersion)
}

// registerReadOnlyResources registers Phase 1 read-only resources
func (s *Server) registerReadOnlyResources() {
	s.registerResource(protocol.Resource{
		URI:         "liza://state",
		Name:        "Current State",
		Description: "Current workspace state (state.yaml)",
		MimeType:    "application/x-yaml",
	})

	s.registerResource(protocol.Resource{
		URI:         "liza://tasks",
		Name:        "Tasks",
		Description: "All tasks in the workspace",
		MimeType:    "application/json",
	})

	s.registerResource(protocol.Resource{
		URI:         "liza://agents",
		Name:        "Agents",
		Description: "All agents in the workspace",
		MimeType:    "application/json",
	})
}

// registerMutationTools registers Phase 2 mutation tools
func (s *Server) registerMutationTools() {
	// liza_add_tasks tool (batch endpoint)
	s.registerTool(protocol.Tool{
		Name:        "liza_add_tasks",
		Description: "Add one or more tasks to the workspace. Requires orchestrator role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"tasks": {
					Type:        "array",
					Description: "Array of task objects. Each object has: id (string, required), desc (string, required), spec (string, required), done (string, required), scope (string, required), priority (number, default 1), depends (array of strings), type (string, default 'coding'), role_pair (string)",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID performing the action (default: orchestrator-1)",
					Default:     "orchestrator-1",
				},
			},
			Required: []string{"tasks"},
		},
	}, s.handleAddTasks)

	// liza_add_task (deprecated compatibility alias — remove after one release)
	s.registerTool(protocol.Tool{
		Name:        "liza_add_task",
		Description: "DEPRECATED: Use liza_add_tasks instead. Add a single task to the workspace.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"id":        {Type: "string", Description: "Unique task ID"},
				"desc":      {Type: "string", Description: "Task description"},
				"spec":      {Type: "string", Description: "Reference to specification file"},
				"done":      {Type: "string", Description: "Completion criteria"},
				"scope":     {Type: "string", Description: "Task scope description"},
				"priority":  {Type: "number", Description: "Task priority (default: 1)", Default: 1},
				"depends":   {Type: "array", Description: "List of task IDs this task depends on"},
				"type":      {Type: "string", Description: "Task type (default: coding)", Default: "coding"},
				"role_pair": {Type: "string", Description: "Role pair for the task (e.g. 'code-planning-pair')"},
				"agent_id":  {Type: "string", Description: "Agent ID performing the action (default: orchestrator-1)", Default: "orchestrator-1"},
			},
			Required: []string{"id", "desc", "spec", "done", "scope"},
		},
	}, s.handleAddTaskCompat)

	// liza_claim_task tool
	s.registerTool(protocol.Tool{
		Name:        "liza_claim_task",
		Description: "Claim an unclaimed task for work. Requires coder, code-planner, epic-planner, or us-writer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to claim",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID claiming the task",
				},
			},
			Required: []string{"task_id", "agent_id"},
		},
	}, s.handleClaimTask)

	// liza_submit_for_review tool
	s.registerTool(protocol.Tool{
		Name:        "liza_submit_for_review",
		Description: "Submit completed work for review after commit SHA validation. Requires coder, code-planner, epic-planner, or us-writer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to submit",
				},
				"commit_sha": {
					Type:        "string",
					Description: "Current task worktree HEAD SHA before rebase (exact match required)",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID submitting the work",
				},
			},
			Required: []string{"task_id", "commit_sha", "agent_id"},
		},
	}, s.handleSubmitForReview)

	// liza_handoff tool
	s.registerTool(protocol.Tool{
		Name:        "liza_handoff",
		Description: "Initiate context-exhaustion handoff for a claimed task. Requires coder, code-planner, epic-planner, or us-writer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to hand off",
				},
				"summary": {
					Type:        "string",
					Description: "Brief summary of completed work",
				},
				"next_action": {
					Type:        "string",
					Description: "Concrete next action for resume",
				},
				"agent_id": {
					Type:        "string",
					Description: "Coder agent ID initiating handoff",
				},
			},
			Required: []string{"task_id", "summary", "next_action", "agent_id"},
		},
	}, s.handleHandoff)

	// liza_submit_verdict tool
	s.registerTool(protocol.Tool{
		Name:        "liza_submit_verdict",
		Description: "Submit review verdict (APPROVED or REJECTED). Requires code-reviewer, code-plan-reviewer, epic-plan-reviewer, or us-reviewer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID being reviewed",
				},
				"verdict": {
					Type:        "string",
					Description: "Review verdict (APPROVED or REJECTED)",
					Enum:        []string{"APPROVED", "REJECTED"},
				},
				"agent_id": {
					Type:        "string",
					Description: "Reviewer agent ID",
				},
				"reason": {
					Type:        "string",
					Description: "Reason for verdict (required for REJECTED)",
				},
			},
			Required: []string{"task_id", "verdict", "agent_id"},
		},
	}, s.handleSubmitVerdict)

	// liza_mark_blocked tool
	s.registerTool(protocol.Tool{
		Name:        "liza_mark_blocked",
		Description: "Mark a task as BLOCKED when work cannot proceed due to unresolvable blocker (spec ambiguity, missing dependency, design conflict). Per blocking protocol: requires reason and 1-3 clarifying questions.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to mark as blocked",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID marking the task blocked (must match task's assigned_to)",
				},
				"reason": {
					Type:        "string",
					Description: "Specific reason for blocking (what is blocking progress)",
				},
				"questions": {
					Type:        "array",
					Description: "1-3 clarifying questions that would unblock if answered",
				},
			},
			Required: []string{"task_id", "agent_id", "reason", "questions"},
		},
	}, s.handleMarkBlocked)

	// liza_release_claim tool
	s.registerTool(protocol.Tool{
		Name:        "liza_release_claim",
		Description: "Release claim on a task",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to release",
				},
				"role": {
					Type:        "string",
					Description: "Role releasing the claim (coder, code-reviewer)",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID releasing the claim",
				},
				"reason": {
					Type:        "string",
					Description: "Reason for releasing",
				},
				"force": {
					Type:        "boolean",
					Description: "Force release even in terminal states",
					Default:     false,
				},
			},
			Required: []string{"task_id", "role", "agent_id"},
		},
	}, s.handleReleaseClaim)

	// liza_supersede_task tool
	s.registerTool(protocol.Tool{
		Name:        "liza_supersede_task",
		Description: "Mark a task as superseded by replacement tasks. Requires orchestrator role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to supersede",
				},
				"replacement_ids": {
					Type:        "array",
					Description: "List of replacement task IDs",
				},
				"reason": {
					Type:        "string",
					Description: "Reason for superseding",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID performing the action",
				},
			},
			Required: []string{"task_id", "reason", "agent_id"},
		},
	}, s.handleSupersede)
}

// registerComplexOperations registers Phase 3 complex operation tools
func (s *Server) registerComplexOperations() {
	// liza_wt_create tool
	s.registerTool(protocol.Tool{
		Name:        "liza_wt_create",
		Description: "Create git worktree for a claimed task",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to create worktree for",
				},
				"fresh": {
					Type:        "boolean",
					Description: "Delete and recreate existing worktree",
					Default:     false,
				},
			},
			Required: []string{"task_id"},
		},
	}, s.handleWtCreate)

	// liza_wt_delete tool
	s.registerTool(protocol.Tool{
		Name:        "liza_wt_delete",
		Description: "Delete git worktree for a task",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to delete worktree for",
				},
			},
			Required: []string{"task_id"},
		},
	}, s.handleWtDelete)

	// liza_wt_merge tool
	s.registerTool(protocol.Tool{
		Name:        "liza_wt_merge",
		Description: "Merge approved task to integration branch. Requires code-reviewer, code-plan-reviewer, epic-plan-reviewer, or us-reviewer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to merge",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID performing the merge",
				},
			},
			Required: []string{"task_id", "agent_id"},
		},
	}, s.handleWtMerge)

	// liza_analyze tool
	s.registerTool(protocol.Tool{
		Name:        "liza_analyze",
		Description: "Run circuit breaker analysis on task patterns",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleAnalyze)

	// liza_update_sprint_metrics tool
	s.registerTool(protocol.Tool{
		Name:        "liza_update_sprint_metrics",
		Description: "Recompute sprint metrics from current state",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleUpdateSprintMetrics)

	// liza_sprint_checkpoint tool
	s.registerTool(protocol.Tool{
		Name:        "liza_sprint_checkpoint",
		Description: "Create sprint checkpoint for human review. Pauses all agents and generates a sprint summary report.",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleSprintCheckpoint)

	// liza_clear_stale_review_claims tool
	s.registerTool(protocol.Tool{
		Name:        "liza_clear_stale_review_claims",
		Description: "Clear expired review leases",
		InputSchema: protocol.InputSchema{
			Type: "object",
		},
	}, s.handleClearStaleReviews)

	// liza_write_checkpoint tool
	s.registerTool(protocol.Tool{
		Name:        "liza_write_checkpoint",
		Description: "Write a pre-execution checkpoint before implementing a task. Required before submission for review. Requires coder, code-planner, epic-planner, or us-writer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to write checkpoint for",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID writing the checkpoint",
				},
				"intent": {
					Type:        "string",
					Description: "Specific, observable intent of the implementation",
				},
				"validation_plan": {
					Type:        "string",
					Description: "Concrete validation command and expected output",
				},
				"files_to_modify": {
					Type:        "array",
					Description: "List of files that will be modified",
				},
				"assumptions": {
					Type:        "array",
					Description: "Tagged assumptions (optional)",
				},
				"risks": {
					Type:        "string",
					Description: "Identified risks (optional)",
				},
				"tdd_not_required": {
					Type:        "string",
					Description: "Justification for why this task does not require new test files (e.g. cosmetic-only change, existing tests cover behavior). If omitted, TDD enforcement applies normally.",
				},
				"scope_extensions": {
					Type:        "array",
					Description: "Files outside task scope that must be modified, with justification. Each entry: {\"file\": \"path\", \"justification\": \"why\"}",
				},
			},
			Required: []string{"task_id", "agent_id", "intent", "validation_plan", "files_to_modify"},
		},
	}, s.handleWriteCheckpoint)

	// liza_set_task_output tool
	s.registerTool(protocol.Tool{
		Name:        "liza_set_task_output",
		Description: "Persist structured task definitions for downstream transition. Requires coder, code-planner, epic-planner, or us-writer role.",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"task_id": {
					Type:        "string",
					Description: "Task ID to set output on",
				},
				"agent_id": {
					Type:        "string",
					Description: "Agent ID setting the output (must be assigned to the task)",
				},
				"output": {
					Type:        "array",
					Description: "Array of output entries, each with: desc (string), done_when (string), scope (string), spec_ref (string, optional)",
				},
			},
			Required: []string{"task_id", "agent_id", "output"},
		},
	}, s.handleSetTaskOutput)

	// liza_delete_agent tool
	s.registerTool(protocol.Tool{
		Name:        "liza_delete_agent",
		Description: "Delete an agent from the workspace",
		InputSchema: protocol.InputSchema{
			Type: "object",
			Properties: map[string]protocol.Property{
				"agent_id": {
					Type:        "string",
					Description: "Agent ID to delete",
				},
				"reason": {
					Type:        "string",
					Description: "Reason for deletion",
				},
				"force": {
					Type:        "boolean",
					Description: "Force deletion even if agent has active tasks",
					Default:     false,
				},
			},
			Required: []string{"agent_id", "reason"},
		},
	}, s.handleDeleteAgent)
}
