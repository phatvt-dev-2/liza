package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// DeleteTaskCommand removes a task from the state database.
// Useful for removing tasks that were created but are no longer needed. Tasks
// in MERGED state cannot be deleted by default (as they represent integrated work).
func DeleteTaskCommand(projectRoot, taskID string, force, deleteWorktree bool, reason string) error {
	// Validate taskID is not empty
	if taskID == "" {
		return fmt.Errorf("task ID required")
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Phase 1: Read and Validate
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

	// Validate deletion is allowed for this status (unless --force)
	now := time.Now().UTC()
	if !force {
		// Check if MERGED
		if task.Status == models.TaskStatusMerged {
			return fmt.Errorf("cannot delete MERGED task %s (use --force if you really want to)", taskID)
		}

		// Check if actively being worked on (CLAIMED with valid lease)
		if task.Status == models.TaskStatusClaimed && task.LeaseExpires != nil && task.LeaseExpires.After(now) {
			return fmt.Errorf("cannot delete task %s in status %s (actively being worked on)", taskID, task.Status)
		}

		// Check if under review
		if task.Status == models.TaskStatusReadyForReview {
			return fmt.Errorf("cannot delete task %s in status %s (under review)", taskID, task.Status)
		}
	}

	// Check for dependent tasks
	var dependentTasks []string
	for _, t := range state.Tasks {
		for _, depID := range t.DependsOn {
			if depID == taskID {
				dependentTasks = append(dependentTasks, t.ID)
			}
		}
	}

	if len(dependentTasks) > 0 && !force {
		return fmt.Errorf("cannot delete task %s: tasks %v depend on it (use --force to override)", taskID, dependentTasks)
	}

	// Warn if forcing delete with dependents
	if len(dependentTasks) > 0 && force {
		fmt.Fprintf(os.Stderr, "Warning: Tasks %v depend on %s and will have dangling dependencies\n", dependentTasks, taskID)
	}

	// Warn if deleting APPROVED task
	if task.Status == models.TaskStatusApproved && !force {
		fmt.Fprintf(os.Stderr, "Warning: The task you are deleting has been implemented, reviewed, and approved. Are you sure you want to delete the task? (y/N): ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			fmt.Println("n")
			return fmt.Errorf("deletion cancelled")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("deletion cancelled")
		}
	}

	// Phase 2: Worktree Handling (outside lock)
	worktreeDeleted := false
	worktreePreserved := false
	var worktreePath string

	if task.Worktree != nil {
		gitWrapper := git.New(lp.ProjectRoot())
		worktreePath = gitWrapper.GetWorktreePath(taskID)

		if deleteWorktree {
			// Delete worktree without prompting
			if err := gitWrapper.RemoveWorktree(taskID); err != nil {
				// Log error but continue - directory might already be gone
				fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
			} else {
				worktreeDeleted = true
			}

			// Delete branch (ignore errors if branch doesn't exist)
			branchName := "task/" + taskID
			_ = gitWrapper.DeleteBranch(branchName)
		} else {
			// Prompt user
			fmt.Fprintf(os.Stderr, "Delete worktree at %s? (y/N): ", worktreePath)
			scanner := bufio.NewScanner(os.Stdin)
			deleteWt := false
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				if answer == "y" || answer == "yes" {
					deleteWt = true
				}
			}
			// If scan failed (EOF, closed stdin), treat as 'n'

			if deleteWt {
				if err := gitWrapper.RemoveWorktree(taskID); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
				} else {
					worktreeDeleted = true
				}

				// Delete branch
				branchName := "task/" + taskID
				_ = gitWrapper.DeleteBranch(branchName)
			} else {
				worktreePreserved = true
			}
		}
	}

	// Phase 3: Atomic State Update
	err = bb.Modify(func(state *models.State) error {
		// Find task in state.Tasks
		taskIndex := -1
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				taskIndex = i
				break
			}
		}

		if taskIndex == -1 {
			return fmt.Errorf("task not found: %s", taskID)
		}

		// Re-check task status hasn't changed (race prevention)
		currentTask := &state.Tasks[taskIndex]
		now := time.Now().UTC()
		if !force {
			if currentTask.Status == models.TaskStatusMerged {
				return fmt.Errorf("cannot delete MERGED task %s (use --force if you really want to)", taskID)
			}

			if currentTask.Status == models.TaskStatusClaimed && currentTask.LeaseExpires != nil && currentTask.LeaseExpires.After(now) {
				return fmt.Errorf("cannot delete task %s in status %s (actively being worked on)", taskID, currentTask.Status)
			}

			if currentTask.Status == models.TaskStatusReadyForReview {
				return fmt.Errorf("cannot delete task %s in status %s (under review)", taskID, currentTask.Status)
			}
		}

		// Clear agent state if task was assigned
		if currentTask.AssignedTo != nil {
			if agent, ok := state.Agents[*currentTask.AssignedTo]; ok {
				if agent.CurrentTask != nil && *agent.CurrentTask == taskID {
					agent.Status = models.AgentStatusIdle
					agent.CurrentTask = nil
					agent.LeaseExpires = nil
					state.Agents[*currentTask.AssignedTo] = agent
				}
			}
		}

		// Remove task from state.Tasks slice
		state.Tasks = append(state.Tasks[:taskIndex], state.Tasks[taskIndex+1:]...)

		// Remove task from sprint.scope.planned if present
		newPlanned := []string{}
		for _, id := range state.Sprint.Scope.Planned {
			if id != taskID {
				newPlanned = append(newPlanned, id)
			}
		}
		state.Sprint.Scope.Planned = newPlanned

		// Remove task from sprint.scope.stretch if present
		newStretch := []string{}
		for _, id := range state.Sprint.Scope.Stretch {
			if id != taskID {
				newStretch = append(newStretch, id)
			}
		}
		state.Sprint.Scope.Stretch = newStretch

		// Add HumanNote for audit trail
		humanNote := models.HumanNote{
			Timestamp: now,
			Message:   fmt.Sprintf("Task %s deleted (was %s): %s", taskID, currentTask.Status, reason),
			For:       "all",
		}
		state.HumanNotes = append(state.HumanNotes, humanNote)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// Success output
	fmt.Printf("Deleted task %s (was %s)\n", taskID, task.Status)
	if worktreeDeleted {
		fmt.Println("Also deleted worktree and branch")
	} else if worktreePreserved {
		fmt.Printf("Worktree at %s preserved (use wt-delete to clean up later)\n", worktreePath)
	}

	return nil
}
