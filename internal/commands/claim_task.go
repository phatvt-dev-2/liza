package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ClaimTaskCommand implements the three-phase claim pattern to prevent TOCTOU races.
// Phase 1: Validate under lock (no mutation)
// Phase 2: Handle worktree outside lock
// Phase 3: Re-validate and commit under lock
func ClaimTaskCommand(projectRoot, taskID, agentID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if agentID == "" {
		return fmt.Errorf("agent ID is required")
	}

	// Setup paths
	lp := paths.New(projectRoot)
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreeDir := filepath.Join(lp.ProjectRoot(), worktreeRel)

	// Get database instance
	bb := db.New(lp.StatePath())

	// --- Phase 1: Validate Under Lock ---
	var taskStatus models.TaskStatus
	var previousAssignee string
	var baseCommit string
	var integrationBranch string
	var leaseDuration int

	// Read state to validate (lock is acquired and released)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Find task
	var task *models.Task
	for i := range state.Tasks {
		if state.Tasks[i].ID == taskID {
			task = &state.Tasks[i]
			break
		}
	}

	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Check task status is claimable
	switch task.Status {
	case models.TaskStatusReady:
		// Check dependencies are satisfied
		if len(task.DependsOn) > 0 {
			var unmet []string
			for _, depID := range task.DependsOn {
				var depTask *models.Task
				for i := range state.Tasks {
					if state.Tasks[i].ID == depID {
						depTask = &state.Tasks[i]
						break
					}
				}
				if depTask == nil || depTask.Status != models.TaskStatusMerged {
					unmet = append(unmet, depID)
				}
			}
			if len(unmet) > 0 {
				return fmt.Errorf("task has unmet dependencies: %v", unmet)
			}
		}
	case models.TaskStatusRejected, models.TaskStatusIntegrationFailed:
		// These are valid source states
		if task.AssignedTo != nil {
			previousAssignee = *task.AssignedTo
		}
	default:
		return fmt.Errorf("task %s is %s (not READY, REJECTED, or INTEGRATION_FAILED)", taskID, task.Status)
	}

	// Check agent isn't already working on another task
	agent, exists := state.Agents[agentID]
	if exists && agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
		return fmt.Errorf("agent %s is already working on task %s", agentID, *agent.CurrentTask)
	}

	// Store values for Phase 2
	taskStatus = task.Status
	integrationBranch = state.Config.IntegrationBranch
	leaseDuration = state.Config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = 1800 // default 30 minutes
	}

	// --- Phase 2: Handle Worktree ---
	gitWrapper := git.New(lp.ProjectRoot())

	// Get base commit for the integration branch
	var err2 error
	baseCommit, err2 = gitWrapper.GetCommitSHA(integrationBranch)
	if err2 != nil {
		return fmt.Errorf("failed to get integration branch commit: %w", err2)
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
		exists, err := gitWrapper.BranchExists(branchName)
		if err != nil {
			return fmt.Errorf("failed to check branch existence: %w", err)
		}
		if exists {
			return fmt.Errorf("branch %s already exists - another claim may be in progress", branchName)
		}

		if _, err := os.Stat(worktreeDir); err == nil {
			return fmt.Errorf("worktree %s already exists for READY task - another claim may be in progress", worktreeRel)
		}

		// Create worktree
		_, err = gitWrapper.CreateWorktree(taskID, integrationBranch)
		if err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		worktreeCreated = true

	case models.TaskStatusRejected:
		if previousAssignee == agentID {
			// Same coder re-claiming - preserve worktree
			fmt.Println("Same coder re-claiming REJECTED task, preserving worktree")
			if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
				return fmt.Errorf("worktree %s missing for REJECTED task (same coder)", worktreeRel)
			}
		} else {
			// Different coder - delete and recreate fresh worktree
			fmt.Println("Different coder claiming REJECTED task, recreating worktree")
			if _, err := os.Stat(worktreeDir); err == nil {
				_ = gitWrapper.RemoveWorktree(taskID)
				_ = gitWrapper.DeleteBranch("task/" + taskID)
				worktreeDeleted = true
			}
			_, err := gitWrapper.CreateWorktree(taskID, integrationBranch)
			if err != nil {
				return fmt.Errorf("failed to create worktree: %w", err)
			}
			worktreeCreated = true
		}

	case models.TaskStatusIntegrationFailed:
		// Preserve worktree for conflict resolution
		fmt.Println("Claiming INTEGRATION_FAILED task, preserving worktree for conflict resolution")
		if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
			return fmt.Errorf("worktree %s missing for INTEGRATION_FAILED task", worktreeRel)
		}
	}

	// --- Phase 3: Re-validate and Commit ---
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	err = bb.Modify(func(state *models.State) error {
		// Re-check task exists and status hasn't changed
		var task *models.Task
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				task = &state.Tasks[i]
				break
			}
		}

		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
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
					var depTask *models.Task
					for i := range state.Tasks {
						if state.Tasks[i].ID == depID {
							depTask = &state.Tasks[i]
							break
						}
					}
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
		// Cleanup on failure — only delete resources we created in this invocation
		if worktreeCreated {
			fmt.Fprintln(os.Stderr, "Cleaning up worktree after failed commit...")
			_ = gitWrapper.RemoveWorktree(taskID)
			// Branch was created with worktree, safe to delete
			_ = gitWrapper.DeleteBranch("task/" + taskID)
		}
		return fmt.Errorf("failed to commit claim: %w", err)
	}

	// Success
	fmt.Printf("IMPLEMENTING: %s by %s (from %s)\n", taskID, agentID, taskStatus)
	fmt.Printf("  worktree: %s\n", worktreeRel)
	fmt.Printf("  base_commit: %s\n", baseCommit)
	fmt.Printf("  lease_expires: %s\n", leaseExpires.Format(time.RFC3339))
	if taskStatus == models.TaskStatusIntegrationFailed {
		fmt.Println("  integration_fix: true")
	}
	if taskStatus == models.TaskStatusRejected && previousAssignee != agentID && previousAssignee != "" {
		if worktreeDeleted {
			fmt.Printf("  previous_assignee: %s (worktree recreated fresh)\n", previousAssignee)
		} else {
			fmt.Printf("  previous_assignee: %s\n", previousAssignee)
		}
	}

	return nil
}
