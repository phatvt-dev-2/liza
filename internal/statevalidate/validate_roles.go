package statevalidate

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/models"
)

// validateRoleNames checks that no agent uses the legacy underscore-form role
// name (e.g. "code_reviewer"). If detected, returns an error directing the
// user to run "liza migrate" for normalization.
func validateRoleNames(state *models.State, _ string, _ bool) error {
	for agentID, agent := range state.Agents {
		if strings.Contains(agent.Role, "_") {
			return fmt.Errorf(
				"agent %s has unmigrated role name %q — run 'liza migrate' to fix",
				agentID, agent.Role,
			)
		}
	}
	return nil
}
