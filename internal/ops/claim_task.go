package ops

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/identity"
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
	Warnings          []string
}

type claimWorktreePhaseResult struct {
	created bool
	deleted bool
}

// ClaimTask implements the three-phase claim pattern to prevent TOCTOU races.
// Phase 1: Validate under lock (no mutation)
// Phase 2: Handle worktree outside lock
// Phase 3: Re-validate and commit under lock
//
// Returns a structured ClaimResult on success. No terminal I/O.
func ClaimTask(projectRoot, taskID, agentID string) (*ClaimResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}

	// Worktree path is deterministic from taskID — always "worktrees/<taskID>".
	// This is the canonical path regardless of task status or prior claim history.
	lp := paths.New(projectRoot)
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreeDir := filepath.Join(lp.ProjectRoot(), worktreeRel)

	bb := db.For(lp.StatePath())

	// --- Phase 1: Validate Under Lock ---
	var taskStatus models.TaskStatus
	var baseCommit string
	var integrationBranch string
	var postWorktreeCmd *string
	var leaseDuration int
	var maxCoderIterations int
	var strategy claimStrategy
	var claimCtx claimContext
	var pipelineTransitions map[models.TaskStatus][]models.TaskStatus

	// Read state to validate (lock is acquired and released)
	state, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	runtimeRole, err := identity.ExtractRole(agentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID %s: %w", agentID, err)
	}

	// Load pipeline resolver for pipeline-aware status resolution
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	if task.RolePair == "" {
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s has no role_pair set", taskID)}
	}

	// Resolve statuses from pipeline config
	pipelineInitial, err := resolver.InitialStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	pipelineRejected, err := resolver.RejectedStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	pipelineExecuting, err := resolver.ExecutingStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	doerRole, err := resolver.DoerRole(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}

	switch task.Status {
	case pipelineInitial:
		strategy = freshClaimStrategy{}
	case pipelineRejected:
		strategy = rejectedClaimStrategy{}
	case models.TaskStatusIntegrationFailed:
		strategy = integrationFixClaimStrategy{}
	default:
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s is %s (not claimable by %s)", taskID, task.Status, runtimeRole)}
	}
	pipelineTransitions = BuildPipelineTransitions(resolver)

	agent, exists := state.Agents[agentID]
	if exists && agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
		return nil, &PreconditionError{Reason: fmt.Sprintf("agent %s is already working on task %s", agentID, *agent.CurrentTask)}
	}

	// Store values for Phase 2
	taskStatus = task.Status
	integrationBranch = state.Config.IntegrationBranch
	postWorktreeCmd = state.Config.PostWorktreeCmd
	leaseDuration = state.Config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = models.DefaultLeaseDurationSeconds
	}
	maxCoderIterations = effectiveCoderIterationLimit(task, state.Config)
	claimCtx = claimContext{
		taskID:            taskID,
		agentID:           agentID,
		taskStatus:        taskStatus,
		targetStatus:      pipelineExecuting,
		worktreeDir:       worktreeDir,
		worktreeRel:       worktreeRel,
		integrationBranch: integrationBranch,
	}
	if err := strategy.validate(task, state, runtimeRole, doerRole, &claimCtx); err != nil {
		return nil, err
	}

	// Enforce coder iteration limits before doing any filesystem work.
	// A REJECTED task at/over the limit is escalated to BLOCKED for orchestrator action.
	if strategy.enforceIterationLimit() && task.Iteration >= maxCoderIterations {
		blockedIteration, blockedLimit, err := enforceRejectedIterationLimit(bb, taskID, agentID, taskStatus, pipelineTransitions)
		if err != nil {
			return nil, fmt.Errorf("failed to enforce iteration limit: %w", err)
		}

		return nil, &PreconditionError{Reason: fmt.Sprintf(
			"task %s reached max iterations (%d/%d) and was transitioned to BLOCKED",
			taskID,
			blockedIteration,
			blockedLimit,
		)}
	}

	// --- Phase 2: Handle Worktree ---
	gitWrapper := git.New(lp.ProjectRoot())

	baseCommit, err = gitWrapper.GetCommitSHA(integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get integration branch commit: %w", err)
	}
	claimCtx.baseCommit = baseCommit

	worktreePhase, err := handleClaimTaskWorktreePhase(
		bb,
		gitWrapper,
		strategy,
		&claimCtx,
	)
	if err != nil {
		return nil, err
	}
	worktreeCreated := worktreePhase.created
	worktreeDeleted := worktreePhase.deleted

	// Run post-worktree command after worktree provisioning.
	// Runs on: fresh claims, rejection reclaims (including same-coder), integration-fix.
	// PostWorktreeCmd is idempotent — safe on existing worktrees, catches prior failures.
	// Non-fatal: warnings are surfaced through ClaimResult for caller visibility.
	var postCmdWarnings []string
	if postWorktreeCmd != nil && strategy.shouldRunPostWorktreeCmd(worktreePhase) {
		if postErr := RunPostWorktreeCmd(*postWorktreeCmd, worktreeDir); postErr != nil {
			warning := fmt.Sprintf("post-worktree-cmd: %v", postErr)
			postCmdWarnings = append(postCmdWarnings, warning)
			log.Printf("WARNING: claim-task %s: %s", taskID, warning)
		}
	}

	// --- Phase 3: Re-validate and Commit ---
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)
	claimCtx.leaseExpires = leaseExpires

	err = bb.Modify(func(state *models.State) error {
		// Re-check task exists and status hasn't changed
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != taskStatus {
			return fmt.Errorf("race condition: task status changed from %s to %s", taskStatus, task.Status)
		}

		// Verify worktree health before committing state (unconditional —
		// concurrent RemoveWorktree can corrupt even pre-existing worktrees).
		if err := gitWrapper.ValidateWorktreeHealth(taskID); err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("worktree not healthy: %v", err)}
		}

		// Re-check dependencies under lock for strategies that require it
		if strategy.requiresDependencyRecheck() {
			if unmet := unmetDependencies(task, state); len(unmet) > 0 {
				return fmt.Errorf("race condition: dependencies changed: %v", unmet)
			}
		}

		// Re-check agent availability
		agent, exists := state.Agents[agentID]
		if exists && agent.CurrentTask != nil && *agent.CurrentTask != "" && *agent.CurrentTask != taskID {
			return fmt.Errorf("race condition: agent %s became busy with %s", agentID, *agent.CurrentTask)
		}

		// Update task
		if err := task.TransitionWith(claimCtx.targetStatus, pipelineTransitions); err != nil {
			return err
		}
		task.AssignedTo = &agentID
		task.LeaseExpires = &leaseExpires

		// Increment iteration (0 -> 1 on first claim, then 2, 3, etc.)
		task.Iteration++

		strategy.mutateTask(task, &claimCtx)
		task.History = append(task.History, strategy.historyEntry(now, &claimCtx))

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
		// Cleanup errors are logged as warnings; the returned error conveys the claim failure.
		// Material inconsistency (orphaned worktree/branch) is flagged for operator attention.
		if worktreeCreated {
			if cleanupErr := gitWrapper.RemoveWorktree(taskID); cleanupErr != nil {
				log.Printf("WARNING: claim-task %s: failed to cleanup worktree after claim failure: %v", taskID, cleanupErr)
			}
			if cleanupErr := gitWrapper.DeleteBranch(paths.TaskBranchPrefix + taskID); cleanupErr != nil {
				log.Printf("WARNING: claim-task %s: failed to cleanup branch after claim failure: %v", taskID, cleanupErr)
			}
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
		PreviousAssignee:  claimCtx.previousAssignee,
		WorktreeRecreated: worktreeDeleted && worktreeCreated,
		Warnings:          postCmdWarnings,
	}, nil
}

func unmetDependencies(task *models.Task, state *models.State) []string {
	var unmet []string
	for _, depID := range task.DependsOn {
		depTask := state.FindTask(depID)
		if depTask == nil || depTask.Status != models.TaskStatusMerged {
			unmet = append(unmet, depID)
		}
	}
	return unmet
}

func handleClaimTaskWorktreePhase(
	bb *db.Blackboard,
	gitWrapper *git.Git,
	strategy claimStrategy,
	ctx *claimContext,
) (claimWorktreePhaseResult, error) {
	return strategy.handleWorktree(bb, gitWrapper, ctx)
}

func handleReadyClaimWorktree(
	gitWrapper *git.Git,
	taskID, integrationBranch, worktreeDir, worktreeRel string,
	cleanupAllowed bool,
) error {
	branchName := paths.TaskBranchPrefix + taskID
	if _, err := gitWrapper.CreateWorktree(taskID, integrationBranch); err == nil {
		return nil
	} else if !isCreateWorktreeConflict(err) {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	if !cleanupAllowed {
		return fmt.Errorf("race condition: concurrent claim already provisioned worktree for READY task")
	}

	branchExists, err := gitWrapper.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}
	if _, statErr := os.Stat(worktreeDir); statErr == nil {
		log.Printf("WARNING: claim-task %s: removing stale worktree %s for READY task", taskID, worktreeRel)
		if cleanupErr := gitWrapper.RemoveWorktree(taskID); cleanupErr != nil {
			return fmt.Errorf("failed to remove stale worktree %s: %w", worktreeRel, cleanupErr)
		}
		_ = gitWrapper.DeleteBranch(branchName)
	} else if branchExists {
		log.Printf("WARNING: claim-task %s: removing stale branch %s for READY task", taskID, branchName)
		if cleanupErr := gitWrapper.DeleteBranch(branchName); cleanupErr != nil {
			return fmt.Errorf("failed to remove stale branch %s: %w", branchName, cleanupErr)
		}
	}

	if _, err := gitWrapper.CreateWorktree(taskID, integrationBranch); err != nil {
		if isCreateWorktreeConflict(err) {
			return fmt.Errorf("race condition: concurrent claim won after stale cleanup")
		}
		return fmt.Errorf("failed to create worktree after stale cleanup: %w", err)
	}

	return nil
}

func isCreateWorktreeConflict(err error) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "worktree already exists") ||
		strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "already checked out")
}

func readyClaimHasStaleResources(gitWrapper *git.Git, taskID, worktreeDir string) (bool, error) {
	branchExists, err := gitWrapper.BranchExists(paths.TaskBranchPrefix + taskID)
	if err != nil {
		return false, fmt.Errorf("failed to check branch existence: %w", err)
	}
	if _, err := os.Stat(worktreeDir); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to stat worktree %s: %w", worktreeDir, err)
	}
	return branchExists, nil
}

func handleRejectedClaimWorktree(
	gitWrapper *git.Git,
	taskID, integrationBranch, previousAssignee, agentID, worktreeDir, worktreeRel string,
) (claimWorktreePhaseResult, error) {
	result := claimWorktreePhaseResult{}
	branchName := paths.TaskBranchPrefix + taskID

	if previousAssignee == agentID {
		// Same coder re-claiming - preserve and validate the existing task worktree.
		if err := validateRejectedSameCoderWorktree(gitWrapper, taskID, worktreeDir, worktreeRel); err != nil {
			return result, err
		}
		return result, nil
	}

	// Different coder - recreate worktree. If replacement creation fails after teardown,
	// restore the previous task worktree so REJECTED state remains recoverable.
	var recoveryRef string
	if _, err := os.Stat(worktreeDir); err == nil {
		branchExists, err := gitWrapper.BranchExists(branchName)
		if err != nil {
			return result, fmt.Errorf("failed to check existing task branch: %w", err)
		}
		if branchExists {
			recoveryRef, err = gitWrapper.GetCommitSHA(branchName)
			if err != nil {
				return result, fmt.Errorf("failed to capture existing task branch for recovery: %w", err)
			}
		}

		if err := gitWrapper.RemoveWorktree(taskID); err != nil {
			return result, fmt.Errorf("failed to remove existing worktree for reassignment: %w", err)
		}

		// RemoveWorktree best-effort deletes the task branch. Ensure a clean branch namespace
		// before creating the replacement.
		_ = gitWrapper.DeleteBranch(branchName)
		result.deleted = true
	}

	if _, err := gitWrapper.CreateWorktree(taskID, integrationBranch); err != nil {
		if result.deleted {
			if recoveryErr := restoreRejectedWorktreeAfterCreateFailure(
				gitWrapper,
				taskID,
				recoveryRef,
				integrationBranch,
			); recoveryErr != nil {
				return result, fmt.Errorf(
					"failed to create worktree: %w; failed to recover previous task worktree: %v",
					err,
					recoveryErr,
				)
			}
		}
		return result, fmt.Errorf("failed to create replacement worktree (previous worktree restored): %w", err)
	}

	result.created = true
	return result, nil
}

func ensureIntegrationFailedWorktreeExists(worktreeDir, worktreeRel string) error {
	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		return fmt.Errorf("worktree %s missing for INTEGRATION_FAILED task", worktreeRel)
	}
	return nil
}

func enforceRejectedIterationLimit(
	bb *db.Blackboard,
	taskID, agentID string,
	expectedStatus models.TaskStatus,
	pipelineTransitions map[models.TaskStatus][]models.TaskStatus,
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

		if err := task.TransitionWith(models.TaskStatusBlocked, pipelineTransitions); err != nil {
			return err
		}
		task.BlockedReason = &blockedReason
		task.BlockedQuestions = questions
		task.LeaseExpires = nil

		if task.AssignedTo != nil {
			previous := *task.AssignedTo
			if a, ok := state.Agents[previous]; ok {
				if a.CurrentTask != nil && *a.CurrentTask == task.ID {
					state.ReleaseAgent(previous)
				}
			}
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

func validateRejectedSameCoderWorktree(
	gitWrapper *git.Git,
	taskID, worktreeDir, worktreeRel string,
) error {
	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		return fmt.Errorf("worktree %s missing for REJECTED task (same coder)", worktreeRel)
	}

	branchName := paths.TaskBranchPrefix + taskID
	branchExists, err := gitWrapper.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check branch %s for REJECTED task (same coder): %w", branchName, err)
	}
	if !branchExists {
		return fmt.Errorf("branch %s missing for REJECTED task (same coder)", branchName)
	}

	if _, err := gitWrapper.GetWorktreeHEAD(taskID); err != nil {
		return fmt.Errorf("worktree %s invalid for REJECTED task (same coder): %w", worktreeRel, err)
	}

	return nil
}

func restoreRejectedWorktreeAfterCreateFailure(
	gitWrapper *git.Git,
	taskID, recoveryRef, fallbackRef string,
) error {
	// Best-effort cleanup of partial replacement artifacts before restoring.
	// Cleanup errors are logged but not propagated - the primary failure is the
	// worktree creation failure that triggered this recovery.
	if cleanupErr := gitWrapper.RemoveWorktree(taskID); cleanupErr != nil {
		log.Printf("WARNING: claim-task recovery %s: failed to cleanup partial worktree: %v", taskID, cleanupErr)
	}
	if cleanupErr := gitWrapper.DeleteBranch(paths.TaskBranchPrefix + taskID); cleanupErr != nil {
		log.Printf("WARNING: claim-task recovery %s: failed to cleanup partial branch: %v", taskID, cleanupErr)
	}

	restoreRef := recoveryRef
	if restoreRef == "" {
		restoreRef = fallbackRef
	}

	_, err := gitWrapper.CreateWorktree(taskID, restoreRef)
	return err
}
