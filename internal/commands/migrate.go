package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/roles"
)

// MigrateCommand normalizes role names in state.yaml from underscore form
// to hyphenated form. Returns (changed, error) where changed indicates
// whether any modifications were made.
func MigrateCommand(statePath string) (bool, error) {
	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return false, fmt.Errorf("failed to read state file: %w", err)
	}

	changed := false
	for agentID, agent := range state.Agents {
		normalized := roles.NormalizeRoleName(agent.Role)
		if normalized != agent.Role {
			agent.Role = normalized
			state.Agents[agentID] = agent
			changed = true
		}
	}

	if !changed {
		return false, nil
	}

	if err := bb.Write(state); err != nil {
		return false, fmt.Errorf("failed to write migrated state: %w", err)
	}

	return true, nil
}
