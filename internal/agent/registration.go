package agent

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// validateIdentity validates agent ID format: {role}-{number}
func validateIdentity(agentID, role string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID required")
	}

	// Split on last hyphen
	lastHyphen := -1
	for i := len(agentID) - 1; i >= 0; i-- {
		if agentID[i] == '-' {
			lastHyphen = i
			break
		}
	}

	if lastHyphen == -1 {
		return fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	idRole := agentID[:lastHyphen]
	numStr := agentID[lastHyphen+1:]

	// Validate number is numeric
	if _, err := strconv.Atoi(numStr); err != nil {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	// Validate role matches
	if idRole != role {
		return fmt.Errorf("agent ID role mismatch (ID=%s, config=%s)", idRole, role)
	}

	return nil
}

// registerAgent registers an agent with collision detection
func registerAgent(bb *db.Blackboard, projectRoot, agentID, role, terminal string, leaseDuration int) error {
	logger := GetLogger()
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	// Single atomic registration - skip STARTING state, go directly to IDLE
	err := bb.Modify(func(state *models.State) error {
		// Check for collision
		if existing, exists := state.Agents[agentID]; exists {
			// Check if lease is still valid
			if existing.LeaseExpires != nil && existing.LeaseExpires.After(now) {
				return fmt.Errorf("agent ID collision: %s already registered with valid lease (expires %s)",
					agentID, existing.LeaseExpires.Format(time.RFC3339))
			}
			logger.Info("Taking over expired agent lease", "agent_id", agentID)
		}

		// Register agent directly as IDLE (atomic operation)
		pid := os.Getpid()
		state.Agents[agentID] = models.Agent{
			Role:         role,
			Status:       models.AgentStatusIdle,
			Heartbeat:    now,
			Terminal:     terminal,
			LeaseExpires: &leaseExpires,
			PID:          pid,
		}

		return nil
	})

	if err != nil {
		return err
	}

	// If code-reviewer: clear stale review claims
	if role == "code-reviewer" {
		if _, err := ops.ClearStaleReviewClaims(projectRoot); err != nil {
			logger.Warn("Failed to clear stale review claims", "error", err, "role", role)
		}
	}

	return nil
}

// unregisterAgent removes an agent from the state
func unregisterAgent(bb *db.Blackboard, agentID string) {
	logger := GetLogger()
	err := bb.Modify(func(state *models.State) error {
		delete(state.Agents, agentID)
		return nil
	})

	if err != nil {
		logger.Warn("Failed to unregister agent", "error", err, "agent_id", agentID)
	}
}

// resetAgentToIdle resets an agent's status to IDLE and clears CurrentTask
func resetAgentToIdle(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		// Reset to IDLE state
		agent.Status = models.AgentStatusIdle
		agent.CurrentTask = nil
		agent.Heartbeat = now

		state.Agents[agentID] = agent
		return nil
	})
}

// resetAgentAfterExit clears transient runtime states after CLI exit while preserving
// explicit command-driven states that are meaningful between loops.
func resetAgentAfterExit(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		switch agent.Status {
		case models.AgentStatusWaiting, models.AgentStatusHandoff:
			if agent.CurrentTask != nil {
				agent.Heartbeat = now
				state.Agents[agentID] = agent
				return nil
			}
			// CurrentTask already cleared — fall through to reset to IDLE
		}

		agent.Status = models.AgentStatusIdle
		agent.CurrentTask = nil
		agent.Heartbeat = now
		state.Agents[agentID] = agent
		return nil
	})
}

// setAgentToPlanningStatus sets a planner agent's status to PLANNING
func setAgentToPlanningStatus(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		// Set to PLANNING state
		agent.Status = models.AgentStatusPlanning
		planning := "planning"
		agent.CurrentTask = &planning
		agent.Heartbeat = now

		// Renew lease
		leaseDuration := state.Config.LeaseDuration
		if leaseDuration <= 0 {
			leaseDuration = models.DefaultLeaseDurationSeconds
		}
		leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)
		agent.LeaseExpires = &leaseExpires

		state.Agents[agentID] = agent
		return nil
	})
}
