package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// RecoverAgentResult contains the outcome of recovering a crashed agent.
type RecoverAgentResult struct {
	AgentID         string
	Role            string
	TaskID          string // empty if no task was associated
	ClaimReleased   bool
	WorktreeRemoved bool
	AgentDeleted    bool
	AlreadyClean    bool // true if agent was not found (idempotent)
	Warnings        []string
}

// RecoverAgent performs full recovery for a crashed agent: releases task claims,
// removes worktrees (for coders), and deletes the agent from state.
// Idempotent: returns AlreadyClean=true if agent not found.
// Without force, refuses if the agent's PID is still alive.
// No terminal I/O.
func RecoverAgent(projectRoot, agentID string, force bool, reason string) (*RecoverAgentResult, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent ID required")
	}
	if reason == "" {
		reason = "agent recovery"
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	// Phase 1: Read — capture agent state
	state, err := bb.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	agent, exists := state.Agents[agentID]
	if !exists {
		return &RecoverAgentResult{
			AgentID:      agentID,
			AlreadyClean: true,
		}, nil
	}

	// PID liveness check
	if !force && agent.PID != 0 && IsProcessAlive(agent.PID) {
		return nil, fmt.Errorf("agent %s is still running with PID %d, use --force to recover", agentID, agent.PID)
	}

	role := agent.Role
	taskID := ""
	if agent.CurrentTask != nil {
		taskID = *agent.CurrentTask
	}

	result := &RecoverAgentResult{
		AgentID: agentID,
		Role:    role,
		TaskID:  taskID,
	}

	// Phase 2: Git side effects (outside lock) — remove worktree for coders
	if role == roles.RuntimeCoder && taskID != "" {
		g := git.New(projectRoot)
		if err := g.RemoveWorktree(taskID); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("worktree removal: %v", err))
		} else {
			result.WorktreeRemoved = true
		}
	}

	// Phase 3: State modify (atomic)
	now := time.Now().UTC()
	err = bb.Modify(func(state *models.State) error {
		// Re-verify agent exists (TOCTOU)
		if _, exists := state.Agents[agentID]; !exists {
			result.AlreadyClean = true
			return nil
		}

		// Release task claim if agent had a task
		if taskID != "" {
			task := state.FindTask(taskID)
			if task != nil {
				switch role {
				case roles.RuntimeCoder:
					released, err := releaseOneClaim(state, task, coderRelease, true, agentID, reason, now)
					if err != nil {
						result.Warnings = append(result.Warnings, fmt.Sprintf("coder claim release: %v", err))
					}
					if released {
						result.ClaimReleased = true
						// Clear worktree reference since we removed it
						task.Worktree = nil
					}
				case roles.RuntimeCodeReviewer:
					released, err := releaseOneClaim(state, task, reviewerRelease, true, agentID, reason, now)
					if err != nil {
						result.Warnings = append(result.Warnings, fmt.Sprintf("reviewer claim release: %v", err))
					}
					if released {
						result.ClaimReleased = true
					}
				}
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("task %s not found in state", taskID))
			}
		}

		// Delete the agent
		delete(state.Agents, agentID)
		result.AgentDeleted = true

		// Add audit trail
		state.HumanNotes = append(state.HumanNotes, models.HumanNote{
			Timestamp: now,
			Message:   fmt.Sprintf("Agent %s recovered (%s): %s", agentID, role, reason),
			For:       agentID,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to recover agent: %w", err)
	}

	// If we raced and agent was already gone
	if result.AlreadyClean {
		return &RecoverAgentResult{
			AgentID:      agentID,
			AlreadyClean: true,
		}, nil
	}

	return result, nil
}
