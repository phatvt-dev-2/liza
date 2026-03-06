package pipeline

import (
	"fmt"
	"slices"

	"github.com/liza-mas/liza/internal/models"
)

// Resolver wraps a PipelineConfig for state resolution queries.
type Resolver struct {
	config *PipelineConfig
}

// NewResolver creates a Resolver from a validated PipelineConfig.
func NewResolver(config *PipelineConfig) *Resolver {
	return &Resolver{config: config}
}

func (r *Resolver) lookupStates(rolePair string) (*RolePairStates, error) {
	rp, ok := r.config.Pipeline.RolePairs[rolePair]
	if !ok {
		return nil, fmt.Errorf("unknown role-pair %q", rolePair)
	}
	return &rp.States, nil
}

// InitialStatus returns the initial state for the given role-pair.
func (r *Resolver) InitialStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Initial), nil
}

// ExecutingStatus returns the executing state for the given role-pair.
func (r *Resolver) ExecutingStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Executing), nil
}

// SubmittedStatus returns the submitted state for the given role-pair.
func (r *Resolver) SubmittedStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Submitted), nil
}

// ReviewingStatus returns the reviewing state for the given role-pair.
func (r *Resolver) ReviewingStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Reviewing), nil
}

// ApprovedStatus returns the approved state for the given role-pair.
func (r *Resolver) ApprovedStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Approved), nil
}

// RejectedStatus returns the rejected state for the given role-pair.
func (r *Resolver) RejectedStatus(rolePair string) (models.TaskStatus, error) {
	s, err := r.lookupStates(rolePair)
	if err != nil {
		return "", err
	}
	return models.TaskStatus(s.Rejected), nil
}

// TransitionMap generates the intra-pair transition map for all role-pairs.
// The fixed intra-pair flow is: initial→executing→submitted→reviewing→approved|rejected,
// with rejected→initial. Cross-cutting meta-states (BLOCKED, ABANDONED, etc.) are not included.
func (r *Resolver) TransitionMap() map[models.TaskStatus][]models.TaskStatus {
	tm := make(map[models.TaskStatus][]models.TaskStatus)
	for _, rp := range r.config.Pipeline.RolePairs {
		s := rp.States
		initial := models.TaskStatus(s.Initial)
		executing := models.TaskStatus(s.Executing)
		submitted := models.TaskStatus(s.Submitted)
		reviewing := models.TaskStatus(s.Reviewing)
		approved := models.TaskStatus(s.Approved)
		rejected := models.TaskStatus(s.Rejected)

		tm[initial] = []models.TaskStatus{executing}
		tm[executing] = []models.TaskStatus{submitted}
		tm[submitted] = []models.TaskStatus{reviewing}
		tm[reviewing] = []models.TaskStatus{approved, rejected}
		tm[rejected] = []models.TaskStatus{initial}
		tm[approved] = []models.TaskStatus{} // terminal within the pair
	}
	return tm
}

// AllDeclaredStates returns all state names declared across all role-pairs.
func (r *Resolver) AllDeclaredStates() []models.TaskStatus {
	var states []models.TaskStatus
	for _, rp := range r.config.Pipeline.RolePairs {
		s := rp.States
		states = append(states,
			models.TaskStatus(s.Initial),
			models.TaskStatus(s.Executing),
			models.TaskStatus(s.Submitted),
			models.TaskStatus(s.Reviewing),
			models.TaskStatus(s.Approved),
			models.TaskStatus(s.Rejected),
		)
	}
	return states
}

// SprintTerminalStates returns the states at which a task is considered complete
// for sprint purposes. For role-pairs whose approved state is the source of a
// transition (i.e., they feed into the next pipeline stage), the approved state
// is sprint-terminal. MERGED is always included as the universal terminal for
// role-pairs at the end of the pipeline.
func (r *Resolver) SprintTerminalStates() []models.TaskStatus {
	// Collect all role-pairs that are sources of transitions.
	transitionSources := make(map[string]bool)
	// From sub-pipeline transitions (2-part refs).
	for _, sp := range r.config.Pipeline.SubPipelines {
		for _, t := range sp.Transitions {
			fromPair, _, _ := parseRef(t.From)
			transitionSources[fromPair] = true
		}
	}
	// From pipeline-transitions (3-part refs).
	for _, t := range r.config.Pipeline.PipelineTransitions {
		_, fromPair, _, err := parse3PartRef(t.From)
		if err == nil {
			transitionSources[fromPair] = true
		}
	}

	var states []models.TaskStatus
	for name, rp := range r.config.Pipeline.RolePairs {
		if transitionSources[name] {
			states = append(states, models.TaskStatus(rp.States.Approved))
		}
	}

	states = append(states, models.TaskStatusMerged)

	// Sort for deterministic output.
	slices.Sort(states)
	return states
}

// RolePairNames returns the sorted names of all role-pairs in the config.
func (r *Resolver) RolePairNames() []string {
	names := make([]string, 0, len(r.config.Pipeline.RolePairs))
	for name := range r.config.Pipeline.RolePairs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// RolePair returns the definition for the named role-pair.
func (r *Resolver) RolePair(name string) (*RolePairDef, error) {
	rp, ok := r.config.Pipeline.RolePairs[name]
	if !ok {
		return nil, fmt.Errorf("unknown role-pair %q", name)
	}
	return &rp, nil
}

// DoerRole returns the doer agent-role key for the given role-pair.
func (r *Resolver) DoerRole(rolePair string) (string, error) {
	rp, ok := r.config.Pipeline.RolePairs[rolePair]
	if !ok {
		return "", fmt.Errorf("unknown role-pair %q", rolePair)
	}
	return rp.Doer, nil
}

// ReviewerRole returns the reviewer agent-role key for the given role-pair.
func (r *Resolver) ReviewerRole(rolePair string) (string, error) {
	rp, ok := r.config.Pipeline.RolePairs[rolePair]
	if !ok {
		return "", fmt.Errorf("unknown role-pair %q", rolePair)
	}
	return rp.Reviewer, nil
}

// Transition returns the transition definition for the given name.
// It searches across all sub-pipelines and pipeline-transitions.
func (r *Resolver) Transition(name string) (*TransitionDef, error) {
	for _, sp := range r.config.Pipeline.SubPipelines {
		for i := range sp.Transitions {
			if sp.Transitions[i].Name == name {
				return &sp.Transitions[i], nil
			}
		}
	}
	for i := range r.config.Pipeline.PipelineTransitions {
		if r.config.Pipeline.PipelineTransitions[i].Name == name {
			return &r.config.Pipeline.PipelineTransitions[i], nil
		}
	}
	return nil, fmt.Errorf("unknown transition %q", name)
}

// AvailableTransitions returns transition names available for a task at the given
// status, excluding transitions already executed.
func (r *Resolver) AvailableTransitions(status models.TaskStatus, transitionsExecuted map[string]bool) []string {
	var available []string
	// Check sub-pipeline transitions.
	for _, sp := range r.config.Pipeline.SubPipelines {
		for _, t := range sp.Transitions {
			if t.Trigger != "manual" {
				continue
			}
			if transitionsExecuted[t.Name] {
				continue
			}
			fromPair, fromPhase, err := parseRef(t.From)
			if err != nil {
				continue
			}
			if r.resolvePhase(fromPair, fromPhase) == status {
				available = append(available, t.Name)
			}
		}
	}
	// Check pipeline-transitions (3-part refs).
	for _, t := range r.config.Pipeline.PipelineTransitions {
		if t.Trigger != "manual" {
			continue
		}
		if transitionsExecuted[t.Name] {
			continue
		}
		if r.resolve3PartPhase(t.From) == status {
			available = append(available, t.Name)
		}
	}
	slices.Sort(available)
	return available
}

// TransitionTargetRolePair returns the target role-pair name for a transition.
// Handles both 2-part refs (sub-pipeline transitions) and 3-part refs (pipeline-transitions).
func (r *Resolver) TransitionTargetRolePair(transitionName string) (string, error) {
	t, err := r.Transition(transitionName)
	if err != nil {
		return "", err
	}
	// Try 3-part ref first (pipeline-transitions use sub-pipeline.role-pair.phase).
	_, toPair, _, err3 := parse3PartRef(t.To)
	if err3 == nil {
		return toPair, nil
	}
	// Fall back to 2-part ref (sub-pipeline transitions use role-pair.phase).
	toPair2, _, err2 := parseRef(t.To)
	if err2 != nil {
		return "", fmt.Errorf("transition %q: invalid to reference: %w", transitionName, err2)
	}
	return toPair2, nil
}

// resolvePhase returns the concrete status for a role-pair + phase combination.
func (r *Resolver) resolvePhase(rolePair, phase string) models.TaskStatus {
	rp, ok := r.config.Pipeline.RolePairs[rolePair]
	if !ok {
		return ""
	}
	switch phase {
	case "initial":
		return models.TaskStatus(rp.States.Initial)
	case "executing":
		return models.TaskStatus(rp.States.Executing)
	case "submitted":
		return models.TaskStatus(rp.States.Submitted)
	case "reviewing":
		return models.TaskStatus(rp.States.Reviewing)
	case "approved":
		return models.TaskStatus(rp.States.Approved)
	case "rejected":
		return models.TaskStatus(rp.States.Rejected)
	default:
		return ""
	}
}

// resolve3PartPhase parses a 3-part ref (sub-pipeline.role-pair.phase) and
// returns the concrete status. The sub-pipeline prefix is stripped since
// role-pair names are globally unique.
func (r *Resolver) resolve3PartPhase(ref string) models.TaskStatus {
	_, rolePair, phase, err := parse3PartRef(ref)
	if err != nil {
		return ""
	}
	return r.resolvePhase(rolePair, phase)
}

// IsDeclaredState checks if a status is declared in the pipeline config.
func (r *Resolver) IsDeclaredState(status models.TaskStatus) bool {
	return slices.Contains(r.AllDeclaredStates(), status)
}
