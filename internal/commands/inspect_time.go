package commands

import (
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// calculateTaskAge returns duration since task was created.
func calculateTaskAge(task *models.Task) time.Duration {
	return time.Since(task.Created)
}

// calculateTimeOnTask returns how long the agent has been on the current task
// by finding the most recent "claimed" event in task history.
func calculateTimeOnTask(task *models.Task) time.Duration {
	if len(task.History) == 0 {
		return 0
	}

	var claimedTime time.Time
	for _, entry := range task.History {
		if entry.Event == models.TaskEventClaimed {
			if claimedTime.IsZero() || entry.Time.After(claimedTime) {
				claimedTime = entry.Time
			}
		}
	}

	if claimedTime.IsZero() {
		return 0
	}

	return time.Since(claimedTime)
}

// calculateTimeSinceHeartbeat returns duration since agent's last heartbeat.
func calculateTimeSinceHeartbeat(agent *models.Agent) time.Duration {
	return time.Since(agent.Heartbeat)
}

// calculateSprintElapsed returns duration since sprint started.
func calculateSprintElapsed(sprint *models.Sprint) time.Duration {
	return time.Since(sprint.Timeline.Started)
}

// calculateSprintRemaining returns duration until deadline.
// Returns negative duration if overdue.
func calculateSprintRemaining(sprint *models.Sprint) time.Duration {
	return time.Until(sprint.Timeline.Deadline)
}
