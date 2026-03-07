package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// formatDuration formats a duration in a human-readable format
// Examples: "45s", "15m", "2h 30m", "1d 1h 45m"
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	negative := d < 0
	if negative {
		d = -d
	}

	seconds := int(d.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	var result string
	if days > 0 {
		hours = hours % 24
		minutes = minutes % 60
		result = fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		minutes = minutes % 60
		result = fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		result = fmt.Sprintf("%dm", minutes)
	} else {
		result = fmt.Sprintf("%ds", seconds)
	}

	if negative {
		return "-" + result
	}
	return result
}

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
		if entry.Event == "claimed" {
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
