package statevalidate

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
)

// validateHandoff checks that every handoff entry has both a summary and a
// next_action. These fields are required for the next agent to resume work
// without re-reading the entire context. Prevents incomplete handoffs that
// would leave successor agents without enough information to continue.
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

// validateDiscovered checks that discovered items have a valid urgency value
// (either "deferred" or "immediate", or empty). Prevents typos and invalid
// urgency levels from entering the backlog where they would be silently ignored
// by the scheduler.
func validateDiscovered(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	for i, disc := range state.Discovered {
		if disc.Urgency != "" && disc.Urgency != "deferred" && disc.Urgency != "immediate" {
			return fmt.Errorf("discovered item %d has invalid urgency '%s' (must be 'deferred' or 'immediate')", i, disc.Urgency)
		}
	}
	return nil
}

// validateAnomalies checks that each anomaly has a valid type and that
// type-specific required detail fields are present (e.g. retry_loop requires
// count and error_pattern; trade_off requires what, why, debt_created).
// Prevents agents from logging anomalies that cannot be analysed by the
// circuit breaker or human reviewers.
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
