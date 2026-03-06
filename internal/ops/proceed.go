package ops

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
)

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

// knownTransitions is the hardcoded transition registry.
// Future step 5 will replace this with YAML config.
var knownTransitions = map[string]transitionDef{
	"code-plan-to-coding": {
		requiredStatus: models.TaskStatusMerged,
		targetStatus:   models.TaskStatusDraft,
		cardinality:    "per-subtask",
	},
}

// Proceed executes a manual inter-pair transition on a source task.
// It creates child tasks from the source's output[] entries and records
// the transition in the source's transitions_executed map.
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

		// Find source task
		task := s.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task %q not found", taskID)
		}

		// Validate source status
		if task.Status != tDef.requiredStatus {
			return fmt.Errorf("task %q must be at %s for transition %q (current: %s)",
				taskID, tDef.requiredStatus, transitionName, task.Status)
		}

		// Check if this is a crash recovery scenario
		alreadyExecuted := task.TransitionsExecuted[transitionName]

		if alreadyExecuted {
			switch tDef.cardinality {
			case "per-subtask":
				// Crash recovery: check if some children are missing
				var missingChildren []int
				for i := range task.Output {
					childID := fmt.Sprintf("%s-%s-%d", taskID, transitionName, i)
					if s.FindTask(childID) == nil {
						missingChildren = append(missingChildren, i)
					}
				}
				if len(missingChildren) == 0 {
					return fmt.Errorf("transition %q already executed on task %q", transitionName, taskID)
				}
				for _, idx := range missingChildren {
					childID := fmt.Sprintf("%s-%s-%d", taskID, transitionName, idx)
					child := buildChildTask(childID, taskID, task.Output[idx], tDef.targetStatus, tDef.targetRolePair, now)
					s.Tasks = append(s.Tasks, child)
					result.ChildTaskIDs = append(result.ChildTaskIDs, childID)
				}
				task.History = append(task.History, models.TaskHistoryEntry{
					Time:  now,
					Event: "transition_crash_recovery",
					Extra: map[string]any{
						"transition":         transitionName,
						"recovered_children": len(missingChildren),
					},
				})
				return nil

			case "one-to-one":
				// Crash recovery: check if child is missing
				childID := fmt.Sprintf("%s-%s", taskID, transitionName)
				if s.FindTask(childID) != nil {
					return fmt.Errorf("transition %q already executed on task %q", transitionName, taskID)
				}
				child := buildOneToOneChild(childID, taskID, task, tDef, now)
				s.Tasks = append(s.Tasks, child)
				result.ChildTaskIDs = append(result.ChildTaskIDs, childID)
				task.History = append(task.History, models.TaskHistoryEntry{
					Time:  now,
					Event: "transition_crash_recovery",
					Extra: map[string]any{
						"transition":         transitionName,
						"recovered_children": 1,
					},
				})
				return nil

			default:
				return fmt.Errorf("transition %q already executed on task %q", transitionName, taskID)
			}
		}

		// Validate and create children based on cardinality
		switch tDef.cardinality {
		case "per-subtask":
			if len(task.Output) == 0 {
				return fmt.Errorf("task %q has no output[] entries for per-subtask transition %q", taskID, transitionName)
			}
			for i, entry := range task.Output {
				if err := validateOutputEntry(entry, i); err != nil {
					return err
				}
			}

			// Mark transition as executed (write this first for crash recovery)
			if task.TransitionsExecuted == nil {
				task.TransitionsExecuted = make(map[string]bool)
			}
			task.TransitionsExecuted[transitionName] = true

			for i, entry := range task.Output {
				childID := fmt.Sprintf("%s-%s-%d", taskID, transitionName, i)
				child := buildChildTask(childID, taskID, entry, tDef.targetStatus, tDef.targetRolePair, now)
				s.Tasks = append(s.Tasks, child)
				result.ChildTaskIDs = append(result.ChildTaskIDs, childID)
			}

		case "one-to-one":
			// Mark transition as executed (write this first for crash recovery)
			if task.TransitionsExecuted == nil {
				task.TransitionsExecuted = make(map[string]bool)
			}
			task.TransitionsExecuted[transitionName] = true

			childID := fmt.Sprintf("%s-%s", taskID, transitionName)
			child := buildOneToOneChild(childID, taskID, task, tDef, now)
			s.Tasks = append(s.Tasks, child)
			result.ChildTaskIDs = append(result.ChildTaskIDs, childID)

		default:
			return fmt.Errorf("unsupported cardinality %q for transition %q", tDef.cardinality, transitionName)
		}

		// Add history entry to source task
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: "transition_executed",
			Extra: map[string]any{
				"transition": transitionName,
				"children":   len(result.ChildTaskIDs),
			},
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("proceed failed: %w", err)
	}

	return result, nil
}

// buildChildTask creates a child task from an output entry.
func buildChildTask(childID, parentID string, entry models.OutputEntry, targetStatus models.TaskStatus, targetRolePair string, now time.Time) models.Task {
	return models.Task{
		ID:          childID,
		Type:        models.TaskTypeCoding,
		RolePair:    targetRolePair,
		Description: entry.Desc,
		Status:      targetStatus,
		Priority:    1,
		ParentTask:  &parentID,
		SpecRef:     entry.SpecRef,
		DoneWhen:    entry.DoneWhen,
		Scope:       entry.Scope,
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
		SpecRef:     parent.SpecRef,
		DoneWhen:    fmt.Sprintf("Complete %s work based on parent task %s", doerName, parentID),
		Scope:       fmt.Sprintf("Based on parent task %s", parentID),
		Created:     now,
		History:     []models.TaskHistoryEntry{},
	}
}

// validateOutputEntry checks that an output entry has all required fields.
func validateOutputEntry(entry models.OutputEntry, index int) error {
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
	return nil
}

// AvailableTransitions returns the available manual transitions for a task.
// For pipeline-configured goals, transitions are read from the frozen config.
// For legacy goals, transitions are read from the hardcoded knownTransitions map.
// Returns nil if no transitions are available.
func AvailableTransitions(task *models.Task, projectRoot string) []string {
	resolver, _, err := loadResolver(projectRoot)
	if err == nil && resolver != nil {
		return resolver.AvailableTransitions(task.Status, task.TransitionsExecuted)
	}
	// Legacy path
	var available []string
	for name, tDef := range knownTransitions {
		if task.Status == tDef.requiredStatus && !task.TransitionsExecuted[name] {
			available = append(available, name)
		}
	}
	return available
}

// resolveTransitionDef looks up a transition definition, trying the pipeline config
// first (if present) and falling back to the legacy knownTransitions map.
func resolveTransitionDef(projectRoot, transitionName string) (transitionDef, error) {
	resolver, cfg, err := loadResolver(projectRoot)
	if err != nil {
		return transitionDef{}, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	if resolver != nil {
		// Pipeline path: look up transition from config
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
			doerDisplay = cfg.Pipeline.AgentRoles[rp.Doer]
		}

		return transitionDef{
			requiredStatus:  fromStatus,
			targetStatus:    toStatus,
			cardinality:     td.Cardinality,
			targetRolePair:  targetPair,
			doerDisplayName: doerDisplay,
		}, nil
	}

	// Legacy path
	td, ok := knownTransitions[transitionName]
	if !ok {
		names := make([]string, 0, len(knownTransitions))
		for name := range knownTransitions {
			names = append(names, name)
		}
		slices.Sort(names)
		return transitionDef{}, fmt.Errorf("unknown transition %q (available: %s)", transitionName, strings.Join(names, ", "))
	}
	return td, nil
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
