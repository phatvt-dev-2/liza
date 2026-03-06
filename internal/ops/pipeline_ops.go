package ops

import (
	"log"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// loadResolver loads the frozen pipeline config for the given project root.
// Returns (nil, nil, nil) for legacy goals (no pipeline.yaml).
func loadResolver(projectRoot string) (*pipeline.Resolver, *pipeline.PipelineConfig, error) {
	cfg, err := pipeline.LoadFrozen(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		return nil, nil, nil
	}
	return pipeline.NewResolver(cfg), cfg, nil
}

// warnSkipRolePair logs a warning when a role-pair is skipped due to a resolver
// error during transition map construction. Should not happen on validated configs.
func warnSkipRolePair(rpName string, err error) {
	log.Printf("WARNING: BuildPipelineTransitions: skipping role-pair %q: %v", rpName, err)
}

// BuildPipelineTransitions creates a complete transition map by merging the
// resolver's intra-pair transitions with cross-cutting meta-state transitions.
func BuildPipelineTransitions(r *pipeline.Resolver) map[models.TaskStatus][]models.TaskStatus {
	tm := r.TransitionMap()

	for _, rpName := range r.RolePairNames() {
		initial, err := r.InitialStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}
		executing, err := r.ExecutingStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}
		submitted, err := r.SubmittedStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}
		reviewing, err := r.ReviewingStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}
		rejected, err := r.RejectedStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}
		approved, err := r.ApprovedStatus(rpName)
		if err != nil {
			warnSkipRolePair(rpName, err)
			continue
		}

		// Cross-cutting additions per lifecycle phase:
		tm[initial] = append(tm[initial], models.TaskStatusAbandoned)
		tm[executing] = append(tm[executing], models.TaskStatusBlocked, initial, models.TaskStatusIntegrationFailed)
		tm[reviewing] = append(tm[reviewing], submitted)
		tm[rejected] = append(tm[rejected], models.TaskStatusBlocked, models.TaskStatusSuperseded, models.TaskStatusAbandoned)
		tm[approved] = append(tm[approved], models.TaskStatusMerged, models.TaskStatusIntegrationFailed)
	}

	// Meta-state transitions
	tm[models.TaskStatusBlocked] = []models.TaskStatus{models.TaskStatusSuperseded, models.TaskStatusAbandoned}

	ifTargets := []models.TaskStatus{models.TaskStatusAbandoned}
	for _, rpName := range r.RolePairNames() {
		executing, err := r.ExecutingStatus(rpName)
		if err != nil {
			log.Printf("WARNING: BuildPipelineTransitions: skipping INTEGRATION_FAILED target for role-pair %q: %v", rpName, err)
			continue
		}
		ifTargets = append(ifTargets, executing)
	}
	tm[models.TaskStatusIntegrationFailed] = ifTargets

	tm[models.TaskStatusMerged] = []models.TaskStatus{}
	tm[models.TaskStatusAbandoned] = []models.TaskStatus{}
	tm[models.TaskStatusSuperseded] = []models.TaskStatus{}

	return tm
}

// SprintTerminalStates returns pipeline-defined sprint-terminal states for a project.
// Returns nil for legacy projects (no pipeline config). On config load error, logs a
// warning and returns nil (falls back to universal terminal states).
func SprintTerminalStates(projectRoot string) []models.TaskStatus {
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		log.Printf("WARNING: failed to load pipeline config for sprint-terminal states: %v", err)
		return nil
	}
	if resolver == nil {
		return nil // Legacy project — no pipeline config
	}
	return resolver.SprintTerminalStates()
}

// allPlannedTasksTerminalForProject checks if all planned tasks are sprint-terminal,
// consulting the pipeline config when available. For legacy projects (no pipeline.yaml),
// falls back to universal terminal states only.
func allPlannedTasksTerminalForProject(s *models.State, projectRoot string) bool {
	return s.AllPlannedTasksTerminalWith(SprintTerminalStates(projectRoot))
}

// LoadResolverForModels loads the pipeline resolver as a models.PipelineResolver.
// Returns nil for legacy projects (no pipeline.yaml) or on error.
func LoadResolverForModels(projectRoot string) models.PipelineResolver {
	resolver, _, err := loadResolver(projectRoot)
	if err != nil || resolver == nil {
		return nil
	}
	return resolver
}

// pipelineBundle holds the resolver, interface, and transition map from a single config load.
// Used to avoid double-parsing pipeline.yaml within a single operation.
type pipelineBundle struct {
	pr          models.PipelineResolver
	transitions map[models.TaskStatus][]models.TaskStatus
}

// loadPipelineBundle loads the pipeline config once and returns the resolver interface
// and pre-built transition map. Returns nil for legacy projects.
func loadPipelineBundle(projectRoot string) *pipelineBundle {
	resolver, _, err := loadResolver(projectRoot)
	if err != nil || resolver == nil {
		return nil
	}
	return &pipelineBundle{
		pr:          resolver,
		transitions: BuildPipelineTransitions(resolver),
	}
}

// initialTaskStatusWithResolver returns the initial task status for a role-pair,
// consulting the pipeline config when available.
func initialTaskStatusWithResolver(rolePair string, resolver *pipeline.Resolver) models.TaskStatus {
	if resolver != nil && rolePair != "" {
		status, err := resolver.InitialStatus(rolePair)
		if err == nil {
			return status
		}
	}
	return initialTaskStatus(rolePair)
}
