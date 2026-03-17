package ops

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
)

// errTransitionAlreadyExecuted is returned by proceedInner when the transition
// has already been fully executed (idempotency guard). This is an expected
// condition in ExecuteAvailableTransitions, not a configuration error.
var errTransitionAlreadyExecuted = errors.New("transition already executed")

// ProceedResult contains the outcome of executing a manual inter-pair transition.
type ProceedResult struct {
	SourceTaskID   string
	TransitionName string
	ChildTaskIDs   []string
}

// transitionDef defines a manual transition between role pairs.
type transitionDef struct {
	// requiredStatus is the source task status required for this transition.
	requiredStatus models.TaskStatus
	// targetStatus is the status assigned to child tasks.
	targetStatus models.TaskStatus
	// cardinality is "per-subtask" or "one-to-one".
	cardinality string
	// targetRolePair is the role-pair set on child tasks (pipeline goals only).
	targetRolePair string
	// doerDisplayName is the display name of the target role-pair's doer (pipeline only).
	// Used for generating child task descriptions in one-to-one transitions.
	doerDisplayName string
}

// Proceed executes a manual inter-pair transition on a source task.
// It creates child tasks from the source's output[] entries and records
// the transition in the source's transitions_executed map.
// Children are added to Sprint.Scope.Planned so they appear in the next sprint.
//
// Preconditions:
//   - Sprint must be COMPLETED
//   - Task must be at the transition's required status
//   - Transition must not already be executed (idempotency guard)
//   - For per-subtask: output[] must be non-empty with valid entries
//
// Crash recovery: if the transition key is already set but some children
// are missing, only the missing children are created.
func Proceed(projectRoot, taskID, transitionName string) (*ProceedResult, error) {
	tDef, err := resolveTransitionDef(projectRoot, transitionName)
	if err != nil {
		return nil, err
	}

	statePath := paths.New(projectRoot).StatePath()
	blackboard := db.For(statePath)

	now := time.Now().UTC()
	result := &ProceedResult{
		SourceTaskID:   taskID,
		TransitionName: transitionName,
	}

	err = blackboard.Modify(func(s *models.State) error {
		// Validate sprint is COMPLETED
		if s.Sprint.Status != models.SprintStatusCompleted {
			return fmt.Errorf("sprint must be COMPLETED before proceeding (current: %s)", s.Sprint.Status)
		}

		if err := proceedInner(s, taskID, transitionName, tDef, now, result); err != nil {
			return err
		}

		// Add children to sprint scope so they appear in the next sprint
		s.Sprint.Scope.Planned = append(s.Sprint.Scope.Planned, result.ChildTaskIDs...)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("proceed failed: %w", err)
	}

	return result, nil
}

// proceedInner is the core transition logic, operating on *models.State directly.
// It has no blackboard dependency and no sprint status check, making it usable
// both from Proceed (human-initiated, with sprint gate) and from
// ExecuteAvailableTransitions (supervisor-initiated, no sprint gate).
//
// The result.ChildTaskIDs slice is appended to with created child task IDs.
func proceedInner(s *models.State, taskID, transitionName string, tDef transitionDef, now time.Time, result *ProceedResult) error {
	task := s.FindTask(taskID)
	if task == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	if task.Status != tDef.requiredStatus {
		return fmt.Errorf("task %q must be at %s for transition %q (current: %s)",
			taskID, tDef.requiredStatus, transitionName, task.Status)
	}

	if task.TransitionsExecuted[transitionName] {
		return recoverCrashedTransition(s, task, taskID, transitionName, tDef, now, result)
	}

	switch tDef.cardinality {
	case "per-subtask":
		if len(task.Output) == 0 {
			return fmt.Errorf("task %q has no output[] entries for per-subtask transition %q", taskID, transitionName)
		}
		for i, entry := range task.Output {
			if err := validateOutputEntry(entry, i, len(task.Output)); err != nil {
				return err
			}
		}
	case "one-to-one":
		if task.SpecRef == "" {
			return fmt.Errorf("task %q has empty spec_ref for one-to-one transition %q", taskID, transitionName)
		}
	default:
		return fmt.Errorf("unsupported cardinality %q for transition %q", tDef.cardinality, transitionName)
	}

	// Write this first for crash recovery
	if task.TransitionsExecuted == nil {
		task.TransitionsExecuted = make(map[string]bool)
	}
	task.TransitionsExecuted[transitionName] = true

	// Pre-compute sibling IDs for DependsOn resolution in per-subtask transitions.
	siblingIDs := make([]string, len(task.Output))
	for i := range task.Output {
		siblingIDs[i] = fmt.Sprintf("%s-%s-%d", taskID, transitionName, i)
	}

	switch tDef.cardinality {
	case "per-subtask":
		for i, entry := range task.Output {
			child := buildChildTask(siblingIDs[i], taskID, entry, tDef.targetStatus, tDef.targetRolePair, siblingIDs, now)
			s.Tasks = append(s.Tasks, child)
			result.ChildTaskIDs = append(result.ChildTaskIDs, siblingIDs[i])
		}
	case "one-to-one":
		childID := fmt.Sprintf("%s-%s", taskID, transitionName)
		child := buildOneToOneChild(childID, taskID, task, tDef, now)
		s.Tasks = append(s.Tasks, child)
		result.ChildTaskIDs = append(result.ChildTaskIDs, childID)
	}

	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventTransitionExecuted,
		Extra: map[string]any{
			"transition": transitionName,
			"children":   len(result.ChildTaskIDs),
		},
	})

	return nil
}

// recoverCrashedTransition handles crash recovery when a transition was already
// marked as executed but some child tasks are missing. Returns
// errTransitionAlreadyExecuted if all children already exist.
func recoverCrashedTransition(s *models.State, task *models.Task, taskID, transitionName string, tDef transitionDef, now time.Time, result *ProceedResult) error {
	switch tDef.cardinality {
	case "per-subtask":
		// Pre-compute sibling IDs for DependsOn resolution.
		siblingIDs := make([]string, len(task.Output))
		for i := range task.Output {
			siblingIDs[i] = fmt.Sprintf("%s-%s-%d", taskID, transitionName, i)
		}
		var missingChildren []int
		for i := range task.Output {
			if s.FindTask(siblingIDs[i]) == nil {
				missingChildren = append(missingChildren, i)
			}
		}
		if len(missingChildren) == 0 {
			return fmt.Errorf("%w: %q on task %q", errTransitionAlreadyExecuted, transitionName, taskID)
		}
		for _, idx := range missingChildren {
			child := buildChildTask(siblingIDs[idx], taskID, task.Output[idx], tDef.targetStatus, tDef.targetRolePair, siblingIDs, now)
			s.Tasks = append(s.Tasks, child)
			result.ChildTaskIDs = append(result.ChildTaskIDs, siblingIDs[idx])
		}
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventTransitionCrashRecov,
			Extra: map[string]any{
				"transition":         transitionName,
				"recovered_children": len(missingChildren),
			},
		})
		return nil

	case "one-to-one":
		childID := fmt.Sprintf("%s-%s", taskID, transitionName)
		if s.FindTask(childID) != nil {
			return fmt.Errorf("%w: %q on task %q", errTransitionAlreadyExecuted, transitionName, taskID)
		}
		child := buildOneToOneChild(childID, taskID, task, tDef, now)
		s.Tasks = append(s.Tasks, child)
		result.ChildTaskIDs = append(result.ChildTaskIDs, childID)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventTransitionCrashRecov,
			Extra: map[string]any{
				"transition":         transitionName,
				"recovered_children": 1,
			},
		})
		return nil

	default:
		return fmt.Errorf("%w: %q on task %q", errTransitionAlreadyExecuted, transitionName, taskID)
	}
}

// ExecuteAvailableTransitions auto-executes pipeline transitions for merged tasks.
// Called by the supervisor after merging approved tasks. For each MERGED task with
// available transitions (per its role-pair's approved status in the pipeline config),
// it creates child tasks in state.Tasks and adds them to Sprint.Scope.Planned
// (with dedup guard for crash recovery idempotency).
//
// This intentionally scans ALL merged tasks, not just newly-merged ones: if the
// supervisor crashes between merge and transition, the next run will pick up the
// pending transition. The idempotency guard in proceedInner (TransitionsExecuted map)
// prevents duplicate child creation.
func ExecuteAvailableTransitions(projectRoot string) ([]ProceedResult, error) {
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	statePath := paths.New(projectRoot).StatePath()
	blackboard := db.For(statePath)

	now := time.Now().UTC()
	var results []ProceedResult

	err = blackboard.Modify(func(s *models.State) error {
		for i := range s.Tasks {
			task := &s.Tasks[i]
			if task.Status != models.TaskStatusMerged {
				continue
			}
			if task.RolePair == "" {
				continue
			}

			// Look up what the approved status was for this task's role-pair.
			// The task is now MERGED, but transitions fire from the approved status.
			approvedStatus, err := resolver.ApprovedStatus(task.RolePair)
			if err != nil {
				log.Printf("WARNING: ExecuteAvailableTransitions: task %s has unknown role-pair %q: %v", task.ID, task.RolePair, err)
				continue
			}

			// Check available transitions at the approved status
			available := resolver.AvailableTransitions(approvedStatus, task.TransitionsExecuted)
			if len(available) == 0 {
				continue
			}

			for _, transitionName := range available {
				// Resolve transition def (allows both manual and auto triggers for supervisor)
				tDef, err := buildTransitionDefFromPipeline(resolver, transitionName)
				if err != nil {
					log.Printf("WARNING: ExecuteAvailableTransitions: task %s transition %q: %v", task.ID, transitionName, err)
					continue
				}

				// Override requiredStatus: the task is MERGED, but the transition
				// definition expects the approved status. We've already validated
				// the role-pair match above, so this is a known override.
				tDef.requiredStatus = models.TaskStatusMerged

				result := ProceedResult{
					SourceTaskID:   task.ID,
					TransitionName: transitionName,
				}

				if err := proceedInner(s, task.ID, transitionName, tDef, now, &result); err != nil {
					// Idempotent skip is expected — only warn on other errors
					if !errors.Is(err, errTransitionAlreadyExecuted) {
						log.Printf("WARNING: ExecuteAvailableTransitions: task %s transition %q: %v", task.ID, transitionName, err)
					}
					continue
				}

				results = append(results, result)
			}
		}

		// Add children to sprint scope (dedup guard: defensive against pre-existing
		// scope entries from partial prior runs)
		for _, r := range results {
			for _, childID := range r.ChildTaskIDs {
				if !slices.Contains(s.Sprint.Scope.Planned, childID) {
					s.Sprint.Scope.Planned = append(s.Sprint.Scope.Planned, childID)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("execute available transitions failed: %w", err)
	}

	return results, nil
}

// buildTransitionDefFromPipeline resolves a transition definition from pipeline config.
// This is the shared helper used by both resolveTransitionDef (with trigger check)
// and ExecuteAvailableTransitions (without trigger check).
func buildTransitionDefFromPipeline(resolver *pipeline.Resolver, transitionName string) (transitionDef, error) {
	td, err := resolver.Transition(transitionName)
	if err != nil {
		return transitionDef{}, err
	}

	fromStatus, err := resolvePhaseRef(resolver, td.From)
	if err != nil {
		return transitionDef{}, fmt.Errorf("invalid from reference in transition %q: %w", transitionName, err)
	}
	toStatus, err := resolvePhaseRef(resolver, td.To)
	if err != nil {
		return transitionDef{}, fmt.Errorf("invalid to reference in transition %q: %w", transitionName, err)
	}
	targetPair, err := resolver.TransitionTargetRolePair(transitionName)
	if err != nil {
		return transitionDef{}, fmt.Errorf("invalid target role-pair in transition %q: %w", transitionName, err)
	}

	// Resolve doer display name for one-to-one child task descriptions.
	var doerDisplay string
	rp, rpErr := resolver.RolePair(targetPair)
	if rpErr == nil {
		doerDisplay = resolver.RoleDisplayName(rp.Doer)
	}

	return transitionDef{
		requiredStatus:  fromStatus,
		targetStatus:    toStatus,
		cardinality:     td.Cardinality,
		targetRolePair:  targetPair,
		doerDisplayName: doerDisplay,
	}, nil
}

// buildChildTask creates a child task from an output entry.
// siblingIDs maps output entry indices to their generated task IDs,
// used to resolve DependsOn index references to actual task IDs.
func buildChildTask(childID, parentID string, entry models.OutputEntry, targetStatus models.TaskStatus, targetRolePair string, siblingIDs []string, now time.Time) models.Task {
	var deps []string
	for _, ref := range entry.DependsOn {
		idx, _ := strconv.Atoi(ref) // validated upstream in validateOutputEntry
		deps = append(deps, siblingIDs[idx])
	}

	return models.Task{
		ID:          childID,
		Type:        models.TaskTypeCoding,
		RolePair:    targetRolePair,
		Description: entry.Desc,
		Status:      targetStatus,
		Priority:    1,
		ParentTask:  &parentID,
		SpecRef:     paths.NormalizeSpecRef(entry.SpecRef),
		PlanRef:     paths.NormalizeSpecRef(entry.PlanRef),
		DoneWhen:    entry.DoneWhen,
		Scope:       entry.Scope,
		DependsOn:   deps,
		Created:     now,
		History:     []models.TaskHistoryEntry{},
	}
}

// buildOneToOneChild creates a child task for a one-to-one transition.
// The parent task itself is the input — no output[] needed. The child's fields
// describe the next phase's work, with spec_ref pointing to the parent's artifact.
func buildOneToOneChild(childID, parentID string, parent *models.Task, tDef transitionDef, now time.Time) models.Task {
	doerName := tDef.doerDisplayName
	if doerName == "" {
		doerName = tDef.targetRolePair
	}

	return models.Task{
		ID:          childID,
		Type:        models.TaskTypeCoding,
		RolePair:    tDef.targetRolePair,
		Description: fmt.Sprintf("%s task for: %s", doerName, parent.Description),
		Status:      tDef.targetStatus,
		Priority:    parent.Priority,
		ParentTask:  &parentID,
		SpecRef:     paths.NormalizeSpecRef(parent.SpecRef),
		PlanRef:     parent.PlanRef, // inherited from parent (set from OutputEntry for per-subtask, propagated for one-to-one)
		DoneWhen:    fmt.Sprintf("Complete %s work based on parent task %s", doerName, parentID),
		Scope:       fmt.Sprintf("Based on parent task %s", parentID),
		Created:     now,
		History:     []models.TaskHistoryEntry{},
	}
}

// validateOutputEntry checks that an output entry has all required fields
// and that DependsOn indices are valid references within [0, totalEntries).
func validateOutputEntry(entry models.OutputEntry, index, totalEntries int) error {
	if entry.Desc == "" {
		return fmt.Errorf("output[%d] missing desc", index)
	}
	if entry.DoneWhen == "" {
		return fmt.Errorf("output[%d] missing done_when", index)
	}
	if entry.Scope == "" {
		return fmt.Errorf("output[%d] missing scope", index)
	}
	if entry.SpecRef == "" {
		return fmt.Errorf("output[%d] missing spec_ref", index)
	}
	return models.ValidateDependsOn(entry.DependsOn, index, totalEntries)
}

// AvailableTransitions returns the available manual transitions for a task.
// Transitions are read from the frozen pipeline config.
// Returns nil if no transitions are available.
func AvailableTransitions(task *models.Task, projectRoot string) []string {
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil
	}
	return resolver.AvailableTransitions(task.Status, task.TransitionsExecuted)
}

// resolveTransitionDef looks up a transition definition from the pipeline config.
// Only manual transitions are allowed — auto transitions are reserved for supervisor
// execution via ExecuteAvailableTransitions.
func resolveTransitionDef(projectRoot, transitionName string) (transitionDef, error) {
	resolver, cfg, err := loadResolver(projectRoot)
	if err != nil {
		return transitionDef{}, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	td, err := resolver.Transition(transitionName)
	if err != nil {
		names := allTransitionNames(cfg)
		return transitionDef{}, fmt.Errorf("unknown transition %q (available: %s)", transitionName, strings.Join(names, ", "))
	}

	// Only manual transitions are executable via liza proceed.
	// Auto transitions are reserved for supervisor execution.
	if td.Trigger != "manual" {
		return transitionDef{}, fmt.Errorf("transition %q has trigger %q; only manual transitions can be executed via proceed", transitionName, td.Trigger)
	}

	return buildTransitionDefFromPipeline(resolver, transitionName)
}

// resolvePhaseRef resolves a phase reference to a concrete TaskStatus.
// Handles both 2-part refs ("role-pair.phase") and 3-part refs
// ("sub-pipeline.role-pair.phase"). For 3-part refs, the sub-pipeline
// prefix is stripped since role-pair names are globally unique.
func resolvePhaseRef(resolver *pipeline.Resolver, ref string) (models.TaskStatus, error) {
	parts := strings.Split(ref, ".")
	var pair, phase string
	switch len(parts) {
	case 2:
		pair, phase = parts[0], parts[1]
	case 3:
		// 3-part ref: sub-pipeline.role-pair.phase — strip sub-pipeline prefix
		pair, phase = parts[1], parts[2]
	default:
		return "", fmt.Errorf("invalid transition reference %q (expected pair.phase or sub-pipeline.pair.phase)", ref)
	}

	if pair == "" || phase == "" {
		return "", fmt.Errorf("invalid transition reference %q: empty components", ref)
	}

	switch phase {
	case "initial":
		return resolver.InitialStatus(pair)
	case "executing":
		return resolver.ExecutingStatus(pair)
	case "submitted":
		return resolver.SubmittedStatus(pair)
	case "reviewing":
		return resolver.ReviewingStatus(pair)
	case "approved":
		return resolver.ApprovedStatus(pair)
	case "rejected":
		return resolver.RejectedStatus(pair)
	default:
		return "", fmt.Errorf("unknown phase %q in reference %q", phase, ref)
	}
}

// allTransitionNames collects all transition names from the pipeline config,
// including both sub-pipeline transitions and pipeline-transitions.
func allTransitionNames(cfg *pipeline.PipelineConfig) []string {
	var names []string
	for _, sp := range cfg.Pipeline.SubPipelines {
		for _, t := range sp.Transitions {
			names = append(names, t.Name)
		}
	}
	for _, t := range cfg.Pipeline.PipelineTransitions {
		names = append(names, t.Name)
	}
	slices.Sort(names)
	return names
}
