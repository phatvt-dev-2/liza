package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// ResolveOrchestratorFromState reads workspace state and returns the registered
// orchestrator's agent ID. When a resolver is provided, agents are matched by
// resolved type ("orchestrator") rather than literal role key, supporting custom
// orchestrator role names defined in pipeline YAML. When resolver is nil, falls
// back to the literal "orchestrator" string match via State.FindOrchestratorID.
// Returns an error if zero or more than one orchestrator is registered.
func ResolveOrchestratorFromState(statePath string, resolver *pipeline.Resolver) (string, error) {
	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return "", fmt.Errorf("reading state to resolve orchestrator: %w", err)
	}
	if resolver != nil {
		return findOrchestratorByType(state, resolver)
	}
	return state.FindOrchestratorID()
}

// findOrchestratorByType iterates agents and finds the one whose role resolves
// to type "orchestrator" via the pipeline resolver.
func findOrchestratorByType(state *models.State, resolver *pipeline.Resolver) (string, error) {
	var found string
	for id, agent := range state.Agents {
		roleType, err := resolver.RoleType(agent.Role)
		if err != nil {
			continue // unknown role, skip
		}
		if roleType == "orchestrator" {
			if found != "" {
				return "", fmt.Errorf("multiple orchestrators registered (%s, %s); pass --agent-id explicitly", found, id)
			}
			found = id
		}
	}
	if found == "" {
		return "", fmt.Errorf("no orchestrator agent registered; pass --agent-id explicitly")
	}
	return found, nil
}
