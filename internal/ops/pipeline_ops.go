package ops

import (
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

// rolePairNames extracts role-pair names from a pipeline config.
func rolePairNames(cfg *pipeline.PipelineConfig) []string {
	names := make([]string, 0, len(cfg.Pipeline.RolePairs))
	for name := range cfg.Pipeline.RolePairs {
		names = append(names, name)
	}
	return names
}

// buildPipelineTransitions creates a complete transition map by merging the
// resolver's intra-pair transitions with cross-cutting meta-state transitions.
func buildPipelineTransitions(r *pipeline.Resolver, cfg *pipeline.PipelineConfig) map[models.TaskStatus][]models.TaskStatus {
	tm := r.TransitionMap()

	for _, rpName := range rolePairNames(cfg) {
		initial, _ := r.InitialStatus(rpName)
		executing, _ := r.ExecutingStatus(rpName)
		submitted, _ := r.SubmittedStatus(rpName)
		reviewing, _ := r.ReviewingStatus(rpName)
		rejected, _ := r.RejectedStatus(rpName)
		approved, _ := r.ApprovedStatus(rpName)

		// Cross-cutting additions per lifecycle phase:
		tm[initial] = append(tm[initial], models.TaskStatusAbandoned)
		tm[executing] = append(tm[executing], models.TaskStatusBlocked, initial)
		tm[reviewing] = append(tm[reviewing], submitted)
		tm[rejected] = append(tm[rejected], models.TaskStatusBlocked, models.TaskStatusSuperseded, models.TaskStatusAbandoned)
		tm[approved] = append(tm[approved], models.TaskStatusMerged, models.TaskStatusIntegrationFailed)
	}

	// Meta-state transitions
	tm[models.TaskStatusBlocked] = []models.TaskStatus{models.TaskStatusSuperseded, models.TaskStatusAbandoned}

	ifTargets := []models.TaskStatus{models.TaskStatusAbandoned}
	for _, rpName := range rolePairNames(cfg) {
		executing, _ := r.ExecutingStatus(rpName)
		ifTargets = append(ifTargets, executing)
	}
	tm[models.TaskStatusIntegrationFailed] = ifTargets

	tm[models.TaskStatusMerged] = []models.TaskStatus{}
	tm[models.TaskStatusAbandoned] = []models.TaskStatus{}
	tm[models.TaskStatusSuperseded] = []models.TaskStatus{}

	return tm
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
