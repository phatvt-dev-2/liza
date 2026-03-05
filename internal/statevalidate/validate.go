package statevalidate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// ValidateStateFile validates the state.yaml file against all schema rules.
// Returns an error with detailed description if validation fails.
func ValidateStateFile(statePath string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	if warnWriter == nil {
		warnWriter = io.Discard
	}

	lizaDir := filepath.Dir(statePath)
	projectRoot := filepath.Dir(lizaDir)

	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	// Load pipeline resolver (nil for legacy goals)
	var resolver *pipeline.Resolver
	cfg, cfgErr := pipeline.LoadFrozen(projectRoot)
	if cfgErr != nil {
		return fmt.Errorf("failed to load pipeline config: %w", cfgErr)
	}
	if cfg != nil {
		resolver = pipeline.NewResolver(cfg)
	}

	validators := []func(*models.State, string, bool) error{
		validateRequiredFields,
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateTaskStates(state, projectRoot, skipSpecFileCheck, resolver)
		},
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateTaskInvariants(state, projectRoot, skipSpecFileCheck, resolver)
		},
		validateDependencies,
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateAgentInvariants(state, projectRoot, skipSpecFileCheck, warnWriter)
		},
		validateHandoff,
		validateDiscovered,
		validateAnomalies,
		validateSprint,
	}

	for _, validator := range validators {
		if err := validator(state, projectRoot, skipSpecFileCheck); err != nil {
			return err
		}
	}

	return nil
}

// ValidateAgentInvariants exposes agent-only invariant checks for package-level tests.
func ValidateAgentInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	if warnWriter == nil {
		warnWriter = io.Discard
	}
	return validateAgentInvariants(state, projectRoot, skipSpecFileCheck, warnWriter)
}

// ValidateAnomalies exposes anomaly validation for package-level tests.
func ValidateAnomalies(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	return validateAnomalies(state, projectRoot, skipSpecFileCheck)
}

func validateRequiredFields(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	if state.Version == 0 {
		return fmt.Errorf("missing required field 'version'")
	}

	if state.Goal.ID == "" {
		return fmt.Errorf("missing required field 'goal'")
	}

	if state.Tasks == nil {
		return fmt.Errorf("missing required field 'tasks'")
	}

	if state.Agents == nil {
		return fmt.Errorf("missing required field 'agents'")
	}

	if state.Config.IntegrationBranch == "" {
		return fmt.Errorf("missing required field 'config'")
	}

	if state.Sprint.ID == "" {
		return fmt.Errorf("missing required field 'sprint'")
	}

	if !skipSpecFileCheck && state.Goal.SpecRef != "" {
		if err := checkSpecFileExists(projectRoot, state.Goal.SpecRef); err != nil {
			return fmt.Errorf("goal %w", err)
		}
	}

	return nil
}

func validateTaskStates(state *models.State, projectRoot string, skipSpecFileCheck bool, resolver *pipeline.Resolver) error {
	for _, task := range state.Tasks {
		statusValid := task.Status.IsValid()
		if !statusValid && resolver != nil {
			// Accept pipeline-declared states and cross-cutting meta-states
			statusValid = resolver.IsDeclaredState(task.Status) || isCrossCuttingState(task.Status)
		}
		if !statusValid {
			return fmt.Errorf("unknown task status '%s' for task %s", task.Status, task.ID)
		}
		if !task.EffectiveType().IsValid() {
			return fmt.Errorf("unknown task type '%s' for task %s", task.Type, task.ID)
		}

		// Pipeline-goal tasks must have a valid role_pair
		if resolver != nil && task.RolePair != "" {
			if _, err := resolver.InitialStatus(task.RolePair); err != nil {
				return fmt.Errorf("task %s has invalid role_pair %q: %w", task.ID, task.RolePair, err)
			}
		}
		if resolver != nil && task.RolePair == "" {
			// Pipeline-declared status without role_pair is invalid
			if resolver.IsDeclaredState(task.Status) {
				return fmt.Errorf("task %s has pipeline-declared status %s but missing role_pair", task.ID, task.Status)
			}
		}
	}
	return nil
}

// isCrossCuttingState returns true for meta-states valid across all role-pairs.
func isCrossCuttingState(status models.TaskStatus) bool {
	switch status {
	case models.TaskStatusBlocked, models.TaskStatusAbandoned,
		models.TaskStatusSuperseded, models.TaskStatusIntegrationFailed,
		models.TaskStatusMerged:
		return true
	}
	return false
}

func validateTaskInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool, resolver *pipeline.Resolver) error {
	// Track agent assignments to prevent duplicates
	assignments := make(map[string][]string) // agent ID -> task IDs
	taskIDs := buildTaskIDSet(state.Tasks)

	// Build the set of pipeline executing statuses for invariant checks
	var pipelineExecutingStatuses []models.TaskStatus
	var pipelineInitialStatuses []models.TaskStatus
	var pipelineSubmittedStatuses []models.TaskStatus
	var pipelineReviewingStatuses []models.TaskStatus
	var pipelineApprovedStatuses []models.TaskStatus
	var pipelineRejectedStatuses []models.TaskStatus
	if resolver != nil {
		for _, rpName := range resolver.RolePairNames() {
			if es, err := resolver.ExecutingStatus(rpName); err == nil {
				pipelineExecutingStatuses = append(pipelineExecutingStatuses, es)
			}
			if is, err := resolver.InitialStatus(rpName); err == nil {
				pipelineInitialStatuses = append(pipelineInitialStatuses, is)
			}
			if ss, err := resolver.SubmittedStatus(rpName); err == nil {
				pipelineSubmittedStatuses = append(pipelineSubmittedStatuses, ss)
			}
			if rs, err := resolver.ReviewingStatus(rpName); err == nil {
				pipelineReviewingStatuses = append(pipelineReviewingStatuses, rs)
			}
			if as, err := resolver.ApprovedStatus(rpName); err == nil {
				pipelineApprovedStatuses = append(pipelineApprovedStatuses, as)
			}
			if rs, err := resolver.RejectedStatus(rpName); err == nil {
				pipelineRejectedStatuses = append(pipelineRejectedStatuses, rs)
			}
		}
	}

	isExecuting := func(s models.TaskStatus) bool {
		if s == models.TaskStatusImplementing || s == models.TaskStatusCodePlanning {
			return true
		}
		for _, es := range pipelineExecutingStatuses {
			if s == es {
				return true
			}
		}
		return false
	}

	isInitial := func(s models.TaskStatus) bool {
		if s == models.TaskStatusDraft || s == models.TaskStatusReady || s == models.TaskStatusDraftCodingPlan {
			return true
		}
		for _, is := range pipelineInitialStatuses {
			if s == is {
				return true
			}
		}
		return false
	}

	isSubmitted := func(s models.TaskStatus) bool {
		if s == models.TaskStatusReadyForReview || s == models.TaskStatusCodingPlanToReview {
			return true
		}
		for _, ss := range pipelineSubmittedStatuses {
			if s == ss {
				return true
			}
		}
		return false
	}

	isReviewing := func(s models.TaskStatus) bool {
		if s == models.TaskStatusReviewing || s == models.TaskStatusReviewingCodingPlan {
			return true
		}
		for _, rs := range pipelineReviewingStatuses {
			if s == rs {
				return true
			}
		}
		return false
	}

	isApproved := func(s models.TaskStatus) bool {
		if s == models.TaskStatusApproved || s == models.TaskStatusCodingPlanApproved {
			return true
		}
		for _, as := range pipelineApprovedStatuses {
			if s == as {
				return true
			}
		}
		return false
	}

	isRejected := func(s models.TaskStatus) bool {
		if s == models.TaskStatusRejected || s == models.TaskStatusCodingPlanRejected {
			return true
		}
		for _, rs := range pipelineRejectedStatuses {
			if s == rs {
				return true
			}
		}
		return false
	}

	for _, task := range state.Tasks {
		// Initial/draft states cannot have assigned_to
		if isInitial(task.Status) && task.AssignedTo != nil {
			return fmt.Errorf("%s task with assigned_to: %s", task.Status, task.ID)
		}

		// Executing states must have assigned_to, worktree, base_commit (unless integration_fix), lease_expires
		if isExecuting(task.Status) {
			if task.AssignedTo == nil {
				return fmt.Errorf("%s task without assigned_to: %s", task.Status, task.ID)
			}
			if task.Worktree == nil {
				return fmt.Errorf("%s task without worktree: %s", task.Status, task.ID)
			}
			if !task.IntegrationFix && task.BaseCommit == nil {
				return fmt.Errorf("%s task without base_commit: %s", task.Status, task.ID)
			}
			if task.LeaseExpires == nil {
				return fmt.Errorf("%s task without lease_expires: %s", task.Status, task.ID)
			}
		}

		// Submitted states must have review_commit
		if isSubmitted(task.Status) && task.ReviewCommit == nil {
			return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
		}

		// Reviewing states must have reviewing_by, review_lease_expires, and review_commit
		if isReviewing(task.Status) {
			if task.ReviewingBy == nil {
				return fmt.Errorf("%s task without reviewing_by: %s", task.Status, task.ID)
			}
			if task.ReviewLeaseExpires == nil {
				return fmt.Errorf("%s task without review_lease_expires: %s", task.Status, task.ID)
			}
			if task.ReviewCommit == nil {
				return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
			}
		}

		// Approved states must have review_commit
		if isApproved(task.Status) && task.ReviewCommit == nil {
			return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
		}

		// MERGED task must NOT have worktree
		if task.Status == models.TaskStatusMerged && task.Worktree != nil {
			return fmt.Errorf("MERGED task still has worktree: %s", task.ID)
		}

		// BLOCKED must have blocked_reason and blocked_questions
		if task.Status == models.TaskStatusBlocked {
			if task.BlockedReason == nil {
				return fmt.Errorf("BLOCKED task without blocked_reason: %s", task.ID)
			}
			if len(task.BlockedQuestions) == 0 {
				return fmt.Errorf("BLOCKED task without blocked_questions: %s", task.ID)
			}
		}

		// Rejected states must have rejection_reason
		if isRejected(task.Status) && task.RejectionReason == nil {
			return fmt.Errorf("%s task without rejection_reason: %s", task.Status, task.ID)
		}

		// SUPERSEDED must have superseded_by and rescope_reason
		if task.Status == models.TaskStatusSuperseded {
			if len(task.SupersededBy) == 0 {
				return fmt.Errorf("SUPERSEDED task without superseded_by: %s", task.ID)
			}
			if task.RescopeReason == nil {
				return fmt.Errorf("SUPERSEDED task without rescope_reason: %s", task.ID)
			}
		}

		// Track assignments for duplicate check (executing tasks count as active)
		if task.AssignedTo != nil && isExecuting(task.Status) {
			agent := *task.AssignedTo
			assignments[agent] = append(assignments[agent], task.ID)
		}

		// Executing task worktree path must exist (only check if projectRoot is not empty to allow tests)
		if isExecuting(task.Status) && task.Worktree != nil && projectRoot != "" {
			wtPath := filepath.Join(projectRoot, *task.Worktree)
			if _, err := os.Stat(wtPath); os.IsNotExist(err) {
				return fmt.Errorf("%s task %s has worktree=%s but directory does not exist", task.Status, task.ID, *task.Worktree)
			}
		}

		if requiresCompletionFields(task.Status, resolver) {
			if task.DoneWhen == "" {
				return fmt.Errorf("non-DRAFT task missing done_when: %s", task.ID)
			}
			if task.SpecRef == "" {
				return fmt.Errorf("non-DRAFT task missing spec_ref: %s", task.ID)
			}
		}

		if task.SpecRef != "" && strings.Contains(task.SpecRef, ".worktrees/") {
			return fmt.Errorf("task %s spec_ref contains worktree prefix (must be repo-relative): %s", task.ID, task.SpecRef)
		}
		if !skipSpecFileCheck && task.SpecRef != "" {
			if err := checkSpecFileExists(projectRoot, task.SpecRef); err != nil {
				return fmt.Errorf("%w (task: %s)", err, task.ID)
			}
		}

		// Task with integration_fix must have INTEGRATION_FAILED in history
		if task.IntegrationFix {
			hasFailedEvent := false
			for _, entry := range task.History {
				if entry.Event == "integration_failed" {
					hasFailedEvent = true
					break
				}
			}
			if !hasFailedEvent {
				return fmt.Errorf("task %s has integration_fix:true but no INTEGRATION_FAILED event in history", task.ID)
			}
		}

		// failed_by must have unique agent IDs
		if len(task.FailedBy) > 0 {
			seen := make(map[string]bool)
			for _, agent := range task.FailedBy {
				if seen[agent] {
					return fmt.Errorf("task %s has duplicate agent IDs in failed_by (manually edit .liza/state.yaml to remove duplicates)", task.ID)
				}
				seen[agent] = true
			}
		}

		// parent_task must reference an existing task
		if task.ParentTask != nil && !taskIDs[*task.ParentTask] {
			return fmt.Errorf("task %s has parent_task referencing non-existent task '%s'", task.ID, *task.ParentTask)
		}

		// output entries must have required fields
		for i, entry := range task.Output {
			if entry.Desc == "" {
				return fmt.Errorf("task %s output[%d] missing desc", task.ID, i)
			}
			if entry.DoneWhen == "" {
				return fmt.Errorf("task %s output[%d] missing done_when", task.ID, i)
			}
			if entry.Scope == "" {
				return fmt.Errorf("task %s output[%d] missing scope", task.ID, i)
			}
			if entry.SpecRef == "" {
				return fmt.Errorf("task %s output[%d] missing spec_ref", task.ID, i)
			}
			if strings.Contains(entry.SpecRef, ".worktrees/") {
				return fmt.Errorf("task %s output[%d] spec_ref contains worktree prefix (must be repo-relative): %s", task.ID, i, entry.SpecRef)
			}
		}
	}

	// Check for duplicate assignments
	for agent, taskIDs := range assignments {
		if len(taskIDs) > 1 {
			return fmt.Errorf("agent %s assigned to multiple active tasks simultaneously: %v", agent, taskIDs)
		}
	}

	return nil
}

func validateDependencies(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	taskIDs := buildTaskIDSet(state.Tasks)

	for _, task := range state.Tasks {
		if len(task.DependsOn) == 0 {
			continue
		}

		// All dependencies must reference existing tasks
		for _, depID := range task.DependsOn {
			if !taskIDs[depID] {
				return fmt.Errorf("task %s has depends_on referencing non-existent task '%s'", task.ID, depID)
			}
		}

		// IMPLEMENTING tasks must have all dependencies MERGED
		if task.Status == models.TaskStatusImplementing {
			var unmet []string
			for _, depID := range task.DependsOn {
				depTask := state.FindTask(depID)
				if depTask != nil && depTask.Status != models.TaskStatusMerged {
					unmet = append(unmet, depID)
				}
			}
			if len(unmet) > 0 {
				return fmt.Errorf("IMPLEMENTING task %s has unmet dependencies: %s (must be MERGED)", task.ID, strings.Join(unmet, ", "))
			}
		}
	}

	for _, task := range state.Tasks {
		if len(task.DependsOn) == 0 {
			continue
		}

		visited := make(map[string]bool)
		if err := checkCircular(task.ID, task.ID, visited, state); err != nil {
			return err
		}
	}

	return nil
}

func checkCircular(start, current string, visited map[string]bool, state *models.State) error {
	task := state.FindTask(current)
	if task == nil || len(task.DependsOn) == 0 {
		return nil
	}

	for _, depID := range task.DependsOn {
		if depID == start {
			return fmt.Errorf("circular dependency detected: %s eventually depends on itself", start)
		}
		if !visited[depID] {
			visited[depID] = true
			if err := checkCircular(start, depID, visited, state); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateAgentInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	now := time.Now().UTC()
	graceDeadline := now.Add(-models.LeaseExpiryGracePeriod)

	for agentID, agent := range state.Agents {
		// WORKING agent must have current_task
		if agent.Status == models.AgentStatusWorking && agent.CurrentTask == nil {
			return fmt.Errorf("agent %s has status WORKING but no current_task assigned", agentID)
		}

		// WORKING agent must have valid lease_expires
		if agent.Status == models.AgentStatusWorking {
			if agent.LeaseExpires == nil {
				return fmt.Errorf("agent %s has status WORKING but no lease_expires", agentID)
			}

			// Check lease expiry with grace period (warning only in original script)
			if agent.LeaseExpires.Before(graceDeadline) {
				// In bash this is a warning, but we'll treat it as an error for stricter validation
				// Could make this configurable if needed
				fmt.Fprintf(warnWriter, "WARNING: Agent %s has status WORKING but lease expired (may be long-running operation)\n", agentID)
			}
		}
	}

	return nil
}

func validateHandoff(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	for taskID, handoff := range state.Handoff {
		if handoff.Summary == "" {
			return fmt.Errorf("handoff entry for task %s missing summary", taskID)
		}
		if handoff.NextAction == "" {
			return fmt.Errorf("handoff entry for task %s missing next_action", taskID)
		}
	}
	return nil
}

func validateDiscovered(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	for i, disc := range state.Discovered {
		if disc.Urgency != "" && disc.Urgency != "deferred" && disc.Urgency != "immediate" {
			return fmt.Errorf("discovered item %d has invalid urgency '%s' (must be 'deferred' or 'immediate')", i, disc.Urgency)
		}
	}
	return nil
}

func validateAnomalies(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	for i, anomaly := range state.Anomalies {
		// Check type is valid
		if !anomaly.IsValidType() {
			return fmt.Errorf("unknown anomaly type '%s' at index %d", anomaly.Type, i)
		}

		// Type-specific detail validation
		switch anomaly.Type {
		case "retry_loop":
			if anomaly.Details["count"] == nil || anomaly.Details["error_pattern"] == nil {
				return fmt.Errorf("retry_loop anomaly at index %d missing required details (count, error_pattern)", i)
			}
		case "trade_off":
			if anomaly.Details["what"] == nil || anomaly.Details["why"] == nil || anomaly.Details["debt_created"] == nil {
				return fmt.Errorf("trade_off anomaly at index %d missing required details (what, why, debt_created)", i)
			}
		case "external_blocker":
			if anomaly.Details["blocker_service"] == nil {
				return fmt.Errorf("external_blocker anomaly at index %d missing required details (blocker_service)", i)
			}
		case "assumption_violated":
			if anomaly.Details["assumption"] == nil || anomaly.Details["reality"] == nil {
				return fmt.Errorf("assumption_violated anomaly at index %d missing required details (assumption, reality)", i)
			}
		case "system_ambiguity":
			if anomaly.Details["protocol_section"] == nil || anomaly.Details["question"] == nil {
				return fmt.Errorf("system_ambiguity anomaly at index %d missing required details (protocol_section, question)", i)
			}
		}
	}
	return nil
}

func validateSprint(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	// Sprint status must be valid
	if !state.Sprint.Status.IsValid() {
		return fmt.Errorf("unknown sprint status '%s'", state.Sprint.Status)
	}

	// Sprint number must be >= 1 (0 is tolerated for legacy pre-multi-sprint state)
	if state.Sprint.Number < 0 {
		return fmt.Errorf("sprint.number must be non-negative (got %d)", state.Sprint.Number)
	}

	// Sprint goal_ref must match goal.id
	if state.Sprint.GoalRef != state.Goal.ID {
		return fmt.Errorf("sprint.goal_ref (%s) does not match goal.id (%s)", state.Sprint.GoalRef, state.Goal.ID)
	}

	taskIDs := buildTaskIDSet(state.Tasks)

	// Sprint scope.planned tasks must exist
	for _, taskID := range state.Sprint.Scope.Planned {
		if !taskIDs[taskID] {
			return fmt.Errorf("sprint.scope.planned references non-existent task '%s'", taskID)
		}
	}

	// Sprint scope.stretch tasks must exist
	for _, taskID := range state.Sprint.Scope.Stretch {
		if !taskIDs[taskID] {
			return fmt.Errorf("sprint.scope.stretch references non-existent task '%s'", taskID)
		}
	}

	// Sprint timeline.started must be set
	if state.Sprint.Timeline.Started.IsZero() {
		return fmt.Errorf("sprint.timeline.started is required")
	}

	// Validate sprint history entries
	if err := validateSprintHistory(state); err != nil {
		return err
	}

	return nil
}

func validateSprintHistory(state *models.State) error {
	seenIDs := make(map[string]bool)
	for i, summary := range state.SprintHistory {
		if summary.ID == "" {
			return fmt.Errorf("sprint_history[%d]: missing id", i)
		}
		if seenIDs[summary.ID] {
			return fmt.Errorf("sprint_history: duplicate sprint id '%s'", summary.ID)
		}
		seenIDs[summary.ID] = true

		if summary.Number < 1 {
			return fmt.Errorf("sprint_history[%d]: number must be >= 1 (got %d)", i, summary.Number)
		}
		if !summary.Status.IsValid() {
			return fmt.Errorf("sprint_history[%d]: invalid status '%s'", i, summary.Status)
		}
		if summary.Started.IsZero() {
			return fmt.Errorf("sprint_history[%d]: missing started time", i)
		}
		if summary.Ended.IsZero() {
			return fmt.Errorf("sprint_history[%d]: missing ended time", i)
		}
	}
	return nil
}

func checkSpecFileExists(projectRoot, specRef string) error {
	specFile := specRef
	if idx := strings.Index(specFile, "#"); idx != -1 {
		specFile = specFile[:idx]
	}
	specPath := specFile
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(projectRoot, specFile)
	}
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("spec_ref file not found: %s", specFile)
	}
	return nil
}

func buildTaskIDSet(tasks []models.Task) map[string]bool {
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids[task.ID] = true
	}
	return ids
}

func requiresCompletionFields(status models.TaskStatus, resolver *pipeline.Resolver) bool {
	// Terminal meta-states don't require completion fields
	if status == models.TaskStatusSuperseded || status == models.TaskStatusAbandoned {
		return false
	}
	// Legacy draft states
	if status == models.TaskStatusDraft || status == models.TaskStatusDraftCodingPlan {
		return false
	}
	// Pipeline initial states (drafts)
	if resolver != nil {
		for _, rpName := range resolver.RolePairNames() {
			if initial, err := resolver.InitialStatus(rpName); err == nil && status == initial {
				return false
			}
		}
	}
	return true
}
