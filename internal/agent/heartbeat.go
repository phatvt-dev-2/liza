package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
)

const (
	// DefaultHeartbeatInterval is the default time between heartbeats
	DefaultHeartbeatInterval = 60 * time.Second
	// DefaultLeaseDuration is the default lease duration
	DefaultLeaseDuration = time.Duration(models.DefaultLeaseDurationSeconds) * time.Second
)

// HeartbeatConfig contains configuration for the heartbeat mechanism
type HeartbeatConfig struct {
	AgentID       string
	StatePath     string
	Interval      time.Duration
	LeaseDuration time.Duration
}

// Heartbeat manages background lease extension for an agent
type Heartbeat struct {
	agentID       string
	bb            *db.Blackboard
	interval      time.Duration
	leaseDuration time.Duration
}

// NewHeartbeat creates a new heartbeat instance
func NewHeartbeat(config HeartbeatConfig) *Heartbeat {
	interval := config.Interval
	if interval == 0 {
		interval = DefaultHeartbeatInterval
	}

	leaseDuration := config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = DefaultLeaseDuration
	}

	return &Heartbeat{
		agentID:       config.AgentID,
		bb:            db.For(config.StatePath),
		interval:      interval,
		leaseDuration: leaseDuration,
	}
}

// Start begins the heartbeat loop, extending the agent's lease periodically
// Returns when the context is cancelled or an unrecoverable error occurs
func (h *Heartbeat) Start(ctx context.Context) error {
	logger := GetLogger()
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := h.beat(); err != nil {
				// Log error but continue (per bash: "|| true")
				// Supervisors can detect stale agents via watch command
				logger.Error("Heartbeat update failed", "error", err, "agent_id", h.agentID)
			}
		}
	}
}

// beat performs a single heartbeat update
func (h *Heartbeat) beat() error {
	now := time.Now().UTC()
	newLease := now.Add(h.leaseDuration)

	return h.bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[h.agentID]
		if !exists {
			return fmt.Errorf("agent %s not found", h.agentID)
		}

		agent.Heartbeat = now
		agent.LeaseExpires = &newLease
		state.Agents[h.agentID] = agent

		return nil
	})
}
