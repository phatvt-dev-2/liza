package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
)

// ResolveOrchestratorFromState reads workspace state and returns the registered
// orchestrator's agent ID. Returns an error if zero or more than one
// orchestrator is registered.
func ResolveOrchestratorFromState(statePath string) (string, error) {
	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return "", fmt.Errorf("reading state to resolve orchestrator: %w", err)
	}
	return state.FindOrchestratorID()
}
