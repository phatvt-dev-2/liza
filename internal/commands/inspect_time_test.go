package commands

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes only",
			duration: 15 * time.Minute,
			want:     "15m",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 30*time.Minute,
			want:     "2h 30m",
		},
		{
			name:     "days, hours, minutes",
			duration: 25*time.Hour + 45*time.Minute,
			want:     "1d 1h 45m",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "less than a minute",
			duration: 30 * time.Second,
			want:     "30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateTaskAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		task    *models.Task
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "recent task",
			task: &models.Task{
				ID:      "task-1",
				Created: now.Add(-2 * time.Hour),
			},
			wantMin: 2 * time.Hour,
			wantMax: 2*time.Hour + time.Second, // Allow small drift
		},
		{
			name: "old task",
			task: &models.Task{
				ID:      "task-2",
				Created: now.Add(-48 * time.Hour),
			},
			wantMin: 48 * time.Hour,
			wantMax: 48*time.Hour + time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTaskAge(tt.task)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateTaskAge() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateTimeOnTask(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		task    *models.Task
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "task claimed 1 hour ago",
			task: &models.Task{
				ID:     "task-1",
				Status: models.TaskStatusClaimed,
				History: []models.TaskHistoryEntry{
					{
						Time:  now.Add(-1 * time.Hour),
						Event: "claimed",
					},
				},
			},
			wantMin: 1 * time.Hour,
			wantMax: 1*time.Hour + time.Second,
		},
		{
			name: "task with no history",
			task: &models.Task{
				ID:      "task-2",
				Status:  models.TaskStatusUnclaimed,
				History: []models.TaskHistoryEntry{},
			},
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTimeOnTask(tt.task)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateTimeOnTask() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateTimeSinceHeartbeat(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		agent   *models.Agent
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "recent heartbeat",
			agent: &models.Agent{
				Role:      "coder",
				Status:    models.AgentStatusWorking,
				Heartbeat: now.Add(-5 * time.Second),
			},
			wantMin: 5 * time.Second,
			wantMax: 6 * time.Second,
		},
		{
			name: "old heartbeat",
			agent: &models.Agent{
				Role:      "coder",
				Status:    models.AgentStatusIdle,
				Heartbeat: now.Add(-10 * time.Minute),
			},
			wantMin: 10 * time.Minute,
			wantMax: 10*time.Minute + time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTimeSinceHeartbeat(tt.agent)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateTimeSinceHeartbeat() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateSprintElapsed(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		sprint  *models.Sprint
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "sprint started 3 hours ago",
			sprint: &models.Sprint{
				ID: "sprint-1",
				Timeline: models.SprintTimeline{
					Started:  now.Add(-3 * time.Hour),
					Deadline: now.Add(5 * time.Hour),
				},
			},
			wantMin: 3 * time.Hour,
			wantMax: 3*time.Hour + time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSprintElapsed(tt.sprint)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateSprintElapsed() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateSprintRemaining(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		sprint  *models.Sprint
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "5 hours remaining",
			sprint: &models.Sprint{
				ID: "sprint-1",
				Timeline: models.SprintTimeline{
					Started:  now.Add(-3 * time.Hour),
					Deadline: now.Add(5 * time.Hour),
				},
			},
			wantMin: 5*time.Hour - time.Second, // Allow small timing drift
			wantMax: 5*time.Hour + time.Second,
		},
		{
			name: "overdue sprint",
			sprint: &models.Sprint{
				ID: "sprint-2",
				Timeline: models.SprintTimeline{
					Started:  now.Add(-10 * time.Hour),
					Deadline: now.Add(-2 * time.Hour),
				},
			},
			wantMin: -2*time.Hour - time.Second,
			wantMax: -2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSprintRemaining(tt.sprint)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateSprintRemaining() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
