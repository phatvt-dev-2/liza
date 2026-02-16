package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// isAgentProcessAlive checks if a process with the given PID is running.
// Note: This is Unix/Linux/macOS specific (uses syscall.Signal(0))
func isAgentProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists without killing it
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// DeleteAgentCommand removes an agent from the state database.
// Useful when an agent has crashed or shutdown uncleanly and needs to be removed
// from the system. By default, refuses to delete agents with active leases or
// current tasks.
func DeleteAgentCommand(projectRoot, agentID string, force bool, reason string) error {
	// Validate agentID is not empty
	if agentID == "" {
		return fmt.Errorf("agent ID required")
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Read state to check agent exists and validate
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Check if agent exists
	agent, exists := state.Agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	// Validate agent lease (unless --force)
	now := time.Now().UTC()
	if !force {
		if agent.LeaseExpires != nil && agent.LeaseExpires.After(now) {
			return fmt.Errorf("agent %s has active lease (expires %v), use --force to delete", agentID, agent.LeaseExpires.Format(time.RFC3339))
		}

		if agent.CurrentTask != nil {
			return fmt.Errorf("agent %s is working on task %s, use --force to delete", agentID, *agent.CurrentTask)
		}

		// Check if agent process is still running (unless --force)
		if agent.PID != 0 && isAgentProcessAlive(agent.PID) {
			// Prompt user for confirmation
			fmt.Fprintf(os.Stderr, "Agent %s is still running with PID %d, do you want to delete the agent from the state file? (y/n): ", agentID, agent.PID)
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("deletion cancelled")
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("deletion cancelled by user")
			}
		}
	}

	// Atomic update
	err = bb.Modify(func(state *models.State) error {
		// Re-check agent exists (race prevention)
		agent, exists := state.Agents[agentID]
		if !exists {
			return fmt.Errorf("agent not found: %s", agentID)
		}

		// Re-validate conditions if not forcing
		now := time.Now().UTC()
		if !force {
			if agent.LeaseExpires != nil && agent.LeaseExpires.After(now) {
				return fmt.Errorf("agent %s has active lease (expires %v), use --force to delete", agentID, agent.LeaseExpires.Format(time.RFC3339))
			}

			if agent.CurrentTask != nil {
				return fmt.Errorf("agent %s is working on task %s, use --force to delete", agentID, *agent.CurrentTask)
			}
		}

		// Delete agent
		delete(state.Agents, agentID)

		// Add HumanNote for audit trail
		humanNote := models.HumanNote{
			Timestamp: now,
			Message:   fmt.Sprintf("Agent %s deleted: %s", agentID, reason),
			For:       agentID,
		}
		state.HumanNotes = append(state.HumanNotes, humanNote)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	// Success output
	fmt.Printf("Deleted agent %s\n", agentID)

	return nil
}
