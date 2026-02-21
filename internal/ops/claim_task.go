package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ClaimResult contains the outcome of a successful task claim.
type ClaimResult struct {
	TaskID            string
	AgentID           string
	SourceStatus      models.TaskStatus
	WorktreeRel       string
	BaseCommit        string
	LeaseExpires      time.Time
	IntegrationFix    bool
	PreviousAssignee  string // empty if none
	WorktreeRecreated bool   // true if old worktree was deleted and new one created
}

// ClaimTask implements the three-phase claim pattern to prevent TOCTOU races.
// Phase 1: Validate under lock (no mutation)
// Phase 2: Handle worktree outside lock
// Phase 3: Re-validate and commit under lock
//
// Returns a structured ClaimResult on success. No terminal I/O.
func ClaimTask(projectRoot, taskID, agentID string) (*ClaimResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	// Worktree path is deterministic from taskID — always "worktrees/<taskID>".
	// This is the canonical path regardless of task status or prior claim history.
	lp := paths.New(projectRoot)
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreeDir := filepath.Join(lp.ProjectRoot(), worktreeRel)

	bb := db.For(lp.StatePath())

	// --- Phase 1: Validate Under Lock ---
	var taskStatus models.TaskStatus
	var previousAssignee string
	var baseCommit string
	var integrationBranch string
	var leaseDuration int
	var maxCoderIterations int

	// Read state to validate (lock is acquired and released)
	state, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	switch task.Status {
	case models.TaskStatusReady:
		// Check dependencies are satisfied
		if len(task.DependsOn) > 0 {
			var unmet []string
			for _, depID := range task.DependsOn {
				depTask := state.FindTask(depID)
				if depTask == nil || depTask.Status != models.TaskStatusMerged {
					unmet = append(unmet, depID)
				}
			}
			if len(unmet) > 0 {
				return nil, fmt.Errorf("task has unmet dependencies: %v", unmet)
			}
		}
	case models.TaskStatusRejected, models.TaskStatusIntegrationFailed:
		// These are valid source states
		if task.AssignedTo != nil {
			previousAssignee = *task.AssignedTo
		}
	default:
		return nil, fmt.Errorf("task %s is %s (not READY, REJECTED, or INTEGRATION_FAILED)", taskID, task.Status)
	}

	agent, exists := state.Agents[agentID]
	if exists && agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
		return nil, fmt.Errorf("agent %s is already working on task %s", agentID, *agent.CurrentTask)
	}

	// Store values for Phase 2
	taskStatus = task.Status
	integrationBranch = state.Config.IntegrationBranch
	leaseDuration = state.Config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = models.DefaultLeaseDurationSeconds
	}
	maxCoderIterations = effectiveCoderIterationLimit(task, state.Config)

	// Enforce coder iteration limits before doing any filesystem work.
	// A REJECTED task at/over the limit is escalated to BLOCKED for planner action.
	if taskStatus == models.TaskStatusRejected && task.Iteration >= maxCoderIterations {
		blockedIteration, blockedLimit, err := enforceRejectedIterationLimit(bb, taskID, agentID, taskStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to enforce iteration limit: %w", err)
		}

		return nil, fmt.Errorf(
			"task %s reached max iterations (%d/%d) and was transitioned to BLOCKED",
			taskID,
			blockedIteration,
			blockedLimit,
		)
	}

	// --- Phase 2: Handle Worktree ---
	gitWrapper := git.New(lp.ProjectRoot())

	// Get base commit for the integration branch
	baseCommit, err = gitWrapper.GetCommitSHA(integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get integration branch commit: %w", err)
	}

	var worktreeCreated bool
	var worktreeDeleted bool

	switch taskStatus {
	case models.TaskStatusReady:
		// New claim - create worktree
		branchName := "task/" + taskID

		// Check if branch or worktree already exists - this indicates a race condition
		// or stale state. Fail fast instead of trying to clean up, as another thread
		// might have just created it.
		branchExists, err := gitWrapper.BranchExists(branchName)
		if err != nil {
			return nil, fmt.Errorf("failed to check branch existence: %w", err)
		}
		if branchExists {
			return nil, fmt.Errorf("branch %s already exists - another claim may be in progress", branchName)
		}

		if _, err := os.Stat(worktreeDir); err == nil {
			return nil, fmt.Errorf("worktree %s already exists for READY task - another claim may be in progress", worktreeRel)
		}

		// Create worktree
		_, err = gitWrapper.CreateWorktree(taskID, integrationBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to create worktree: %w", err)
		}
		worktreeCreated = true

	case models.TaskStatusRejected:
		if previousAssignee == agentID {
			// Same coder re-claiming - preserve worktree
			if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
				return nil, fmt.Errorf("worktree %s missing for REJECTED task (same coder)", worktreeRel)
			}
		} else {
			// Different coder - delete and recreate fresh worktree
			if _, err := os.Stat(worktreeDir); err == nil {
				_ = gitWrapper.RemoveWorktree(taskID)
				_ = gitWrapper.DeleteBranch("task/" + taskID)
				worktreeDeleted = true
			}
			_, err := gitWrapper.CreateWorktree(taskID, integrationBranch)
			if err != nil {
				return nil, fmt.Errorf("failed to create worktree: %w", err)
			}
			worktreeCreated = true
		}

	case models.TaskStatusIntegrationFailed:
		// Preserve worktree for conflict resolution
		if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("worktree %s missing for INTEGRATION_FAILED task", worktreeRel)
		}
	}

	// --- Phase 3: Re-validate and Commit ---
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	err = bb.Modify(func(state *models.State) error {
		// Re-check task exists and status hasn't changed
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != taskStatus {
			return fmt.Errorf("race condition: task status changed from %s to %s", taskStatus, task.Status)
		}

		// Verify worktree exists on disk before committing state
		if taskStatus == models.TaskStatusReady && worktreeCreated {
			worktreePath := filepath.Join(lp.ProjectRoot(), worktreeRel)
			if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
				return fmt.Errorf("worktree directory does not exist: %s", worktreePath)
			}
		}

		// For READY: re-check dependencies
		if taskStatus == models.TaskStatusReady {
			if len(task.DependsOn) > 0 {
				var unmet []string
				for _, depID := range task.DependsOn {
					depTask := state.FindTask(depID)
					if depTask == nil || depTask.Status != models.TaskStatusMerged {
						unmet = append(unmet, depID)
					}
				}
				if len(unmet) > 0 {
					return fmt.Errorf("race condition: dependencies changed: %v", unmet)
				}
			}
		}

		// Re-check agent availability
		agent, exists := state.Agents[agentID]
		if exists && agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
			return fmt.Errorf("race condition: agent %s became busy with %s", agentID, *agent.CurrentTask)
		}

		// Build event description
		event := "claimed"
		if taskStatus == models.TaskStatusRejected {
			if previousAssignee == agentID {
				event = "reclaimed_after_rejection"
			} else {
				event = "reassigned_after_rejection"
			}
		} else if taskStatus == models.TaskStatusIntegrationFailed {
			event = "claimed_for_integration_fix"
		}

		// Update task
		if err := task.Transition(models.TaskStatusImplementing); err != nil {
			return err
		}
		task.AssignedTo = &agentID
		task.LeaseExpires = &leaseExpires

		// Increment iteration (0 -> 1 on first claim, then 2, 3, etc.)
		task.Iteration++

		// Different updates based on source state
		if taskStatus == models.TaskStatusIntegrationFailed {
			// Set integration_fix flag
			task.IntegrationFix = true
		} else if taskStatus == models.TaskStatusRejected && previousAssignee != agentID {
			// Different coder: reset review_cycles_current, update base_commit and worktree
			task.Worktree = &worktreeRel
			task.BaseCommit = &baseCommit
			task.ReviewCyclesCurrent = 0
		} else if taskStatus == models.TaskStatusRejected && previousAssignee == agentID {
			// Same coder: preserve base_commit and worktree (already set)
		} else {
			// READY: new claim with fresh worktree and base_commit
			task.Worktree = &worktreeRel
			task.BaseCommit = &baseCommit
		}

		// Add history entry
		agentPtr := &agentID
		historyEntry := models.TaskHistoryEntry{
			Time:  now,
			Event: event,
			Agent: agentPtr,
		}
		if taskStatus == models.TaskStatusRejected && previousAssignee != agentID && previousAssignee != "" {
			historyEntry.PreviousAssignee = &previousAssignee
		}
		task.History = append(task.History, historyEntry)

		// Update agent
		if !exists {
			state.Agents[agentID] = models.Agent{}
		}
		agent = state.Agents[agentID]
		agent.Status = models.AgentStatusWorking
		agent.CurrentTask = &taskID
		agent.LeaseExpires = &leaseExpires
		agent.Heartbeat = now
		state.Agents[agentID] = agent

		return nil
	})

	if err != nil {
		// Cleanup on failure — only delete resources we created in this invocation.
		// Cleanup errors are best-effort; the returned error conveys the claim failure.
		// Callers (agent, commands) should log this if operational visibility is needed.
		if worktreeCreated {
			_ = gitWrapper.RemoveWorktree(taskID)
			_ = gitWrapper.DeleteBranch("task/" + taskID)
		}
		return nil, fmt.Errorf("failed to commit claim: %w", err)
	}

	return &ClaimResult{
		TaskID:            taskID,
		AgentID:           agentID,
		SourceStatus:      taskStatus,
		WorktreeRel:       worktreeRel,
		BaseCommit:        baseCommit,
		LeaseExpires:      leaseExpires,
		IntegrationFix:    taskStatus == models.TaskStatusIntegrationFailed,
		PreviousAssignee:  previousAssignee,
		WorktreeRecreated: worktreeDeleted && worktreeCreated,
	}, nil
}

func enforceRejectedIterationLimit(
	bb *db.Blackboard,
	taskID, agentID string,
	expectedStatus models.TaskStatus,
) (int, int, error) {
	now := time.Now().UTC()
	blockedIteration := 0
	blockedLimit := 0

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		if task.Status != expectedStatus {
			return fmt.Errorf("race condition: task status changed from %s to %s", expectedStatus, task.Status)
		}

		blockedLimit = effectiveCoderIterationLimit(task, state.Config)
		if task.Iteration < blockedLimit {
			return fmt.Errorf(
				"race condition: task iteration no longer at limit (%d/%d)",
				task.Iteration,
				blockedLimit,
			)
		}

		blockedIteration = task.Iteration
		blockedReason := iterationLimitBlockedReason(task.Iteration, blockedLimit)
		questions := defaultIterationLimitBlockedQuestions()

		if err := task.Transition(models.TaskStatusBlocked); err != nil {
			return err
		}
		task.BlockedReason = &blockedReason
		task.BlockedQuestions = questions
		task.LeaseExpires = nil

		if task.AssignedTo != nil {
			previous := *task.AssignedTo
			state.ReleaseAgent(previous)
		}
		task.AssignedTo = nil

		agentPtr := &agentID
		reasonPtr := &blockedReason
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  "blocked",
			Agent:  agentPtr,
			Reason: reasonPtr,
		})

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return blockedIteration, blockedLimit, nil
}
