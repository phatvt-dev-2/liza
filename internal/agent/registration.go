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
	"github.com/liza-mas/liza/internal/pipeline"
)

// validateIdentity validates agent ID format: {role}-{number}
func validateIdentity(agentID, role string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID required")
	}

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

	if _, err := strconv.Atoi(numStr); err != nil {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	if idRole != role {
		return fmt.Errorf("agent ID role mismatch (ID=%s, config=%s)", idRole, role)
	}

	return nil
}

// registerAgent registers an agent with collision detection.
// provider identifies the CLI provider (e.g. "claude", "codex") and is persisted
// for review quorum provider-diversity checks.
// resolver is used for role classification (singularity, reviewer detection).
func registerAgent(bb *db.Blackboard, projectRoot, agentID, role, terminal string, leaseDuration int, provider string, resolver *pipeline.Resolver) error {
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

		// Singularity check via resolver: at most N instances per role.
		if resolver != nil {
			maxInst, err := resolver.MaxInstances(role)
			if err == nil && maxInst > 0 {
				liveCount := 0
				for id, agent := range state.Agents {
					if id != agentID && agent.Role == role {
						if agent.LeaseExpires != nil && agent.LeaseExpires.After(now) {
							liveCount++
						}
					}
				}
				if liveCount >= maxInst {
					return fmt.Errorf("role %s already has %d live agent(s) (max %d); only %d instance(s) allowed",
						role, liveCount, maxInst, maxInst)
				}
			}
		}

		// Register agent directly as IDLE (atomic operation)
		pid := os.Getpid()
		state.Agents[agentID] = models.Agent{
			Role:         role,
			Status:       models.AgentStatusIdle,
			Heartbeat:    now,
			Terminal:     terminal,
			Provider:     provider,
			LeaseExpires: &leaseExpires,
			PID:          pid,
		}

		return nil
	})

	if err != nil {
		return err
	}

	// If reviewer role: clear stale review claims
	if resolver != nil {
		roleType, rtErr := resolver.RoleType(role)
		if rtErr == nil && roleType == "reviewer" {
			if _, err := ops.ClearStaleReviewClaims(projectRoot); err != nil {
				logger.Warn("Failed to clear stale review claims", "error", err, "role", role)
			}
		}
	}

	return nil
}

// unregisterAgent releases any task claim held by the agent, then removes
// the agent from state. Both operations happen in a single atomic modify
// so that an interrupt between them cannot leave a stuck task.
func unregisterAgent(bb *db.Blackboard, agentID, projectRoot string) {
	logger := GetLogger()
	now := time.Now().UTC()

	// Load pipeline config outside the lock to avoid disk I/O under bb.Modify
	pipelineTransitions, resolver := loadPipelineForRelease(projectRoot)

	err := bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return nil
		}

		// Release task claim if agent held one
		if agent.CurrentTask != nil {
			taskID := *agent.CurrentTask
			if task := state.FindTask(taskID); task != nil {
				releaseTaskClaim(state, task, agent.Role, agentID, pipelineTransitions, resolver, now)
			}
		}

		delete(state.Agents, agentID)
		return nil
	})

	if err != nil {
		logger.Warn("Failed to unregister agent", "error", err, "agent_id", agentID)
	}
}

// releaseTaskClaim transitions a task back to its unclaimed status and clears
// the claim fields. Best-effort: logs warnings instead of failing.
// pipelineTransitions and resolver are pre-loaded by the caller (outside bb.Modify)
// to avoid disk I/O under the state lock.
// Uses resolver.RoleType() for doer/reviewer classification.
func releaseTaskClaim(state *models.State, task *models.Task, role, agentID string, pipelineTransitions map[models.TaskStatus][]models.TaskStatus, resolver *pipeline.Resolver, now time.Time) {
	logger := GetLogger()
	reason := "agent interrupted"

	// Resolve pipeline-aware statuses (shared logic with ops.ReleaseClaim)
	activeExecuting, releasedInitial, activeReviewing, releasedSubmitted := ops.ResolveReleaseStatuses(task, resolver)

	transitionTask := func(to models.TaskStatus) {
		if pipelineTransitions == nil {
			logger.Warn("Cannot transition task on unregister: pipeline transitions not loaded", "task_id", task.ID)
			return
		}
		if err := task.TransitionWith(to, pipelineTransitions); err != nil {
			logger.Warn("Failed to transition task on unregister", "task_id", task.ID, "error", err)
		}
	}

	// Classify role using resolver for doer/reviewer determination.
	roleType := ""
	if resolver != nil {
		if rt, err := resolver.RoleType(role); err == nil {
			roleType = rt
		}
	}

	switch roleType {
	case "doer":
		if task.Status == activeExecuting {
			transitionTask(releasedInitial)
		}
		task.AssignedTo = nil
		task.LeaseExpires = nil

	case "reviewer":
		if task.Status == activeReviewing {
			transitionTask(releasedSubmitted)
		}
		task.ReviewingBy = nil
		task.ReviewLeaseExpires = nil

	default:
		return
	}

	state.ReleaseAgent(agentID)

	task.History = append(task.History, models.TaskHistoryEntry{
		Time:   now,
		Event:  models.TaskEventClaimReleased,
		Agent:  &agentID,
		Reason: &reason,
	})
}

// loadPipelineForRelease loads pipeline resolver and transitions, logging
// warnings on failure. Returns nil values when pipeline config is unreadable.
func loadPipelineForRelease(projectRoot string) (map[models.TaskStatus][]models.TaskStatus, *pipeline.Resolver) {
	if projectRoot == "" {
		return nil, nil
	}
	cfg, err := pipeline.LoadFrozen(projectRoot)
	if err != nil {
		GetLogger().Warn("Failed to load pipeline config for claim release", "error", err)
		return nil, nil
	}
	resolver := pipeline.NewResolver(cfg)
	return ops.BuildPipelineTransitions(resolver), resolver
}

// resetAgentToIdle resets an agent's status to IDLE and clears CurrentTask
func resetAgentToIdle(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return &errors.NotFoundError{Entity: "agent", ID: agentID}
		}

		agent.Status = models.AgentStatusIdle
		agent.CurrentTask = nil
		agent.Heartbeat = now

		state.Agents[agentID] = agent
		return nil
	})
}

// resetAgentAfterExit clears transient runtime states after CLI exit while preserving
// explicit command-driven states that are meaningful between loops.
func resetAgentAfterExit(bb *db.Blackboard, agentID, projectRoot string) error {
	now := time.Now().UTC()

	// Load pipeline config outside the lock to avoid disk I/O under bb.Modify
	pipelineTransitions, resolver := loadPipelineForRelease(projectRoot)

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

		// Release any held task claim before clearing CurrentTask
		if agent.CurrentTask != nil {
			if task := state.FindTask(*agent.CurrentTask); task != nil {
				releaseTaskClaim(state, task, agent.Role, agentID, pipelineTransitions, resolver, now)
			}
		}

		agent.Status = models.AgentStatusIdle
		agent.CurrentTask = nil
		agent.Heartbeat = now
		state.Agents[agentID] = agent
		return nil
	})
}

// setAgentToOrchestratingStatus sets an orchestrator agent's status to PLANNING
func setAgentToOrchestratingStatus(bb *db.Blackboard, agentID string) error {
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
