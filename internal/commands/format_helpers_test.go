package commands

import (
	"strings"
	"testing"
)

func TestFormatKeyValue(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]any
		wantContain []string // Check that output contains these lines
	}{
		{
			name: "simple fields",
			data: map[string]any{
				"id":     "task-1",
				"status": "IMPLEMENTING_CODE",
				"count":  42,
			},
			wantContain: []string{"id: task-1", "status: IMPLEMENTING_CODE", "count: 42"},
		},
		{
			name: "with nil value",
			data: map[string]any{
				"id":     "task-1",
				"status": nil,
			},
			wantContain: []string{"id: task-1", "status: <none>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKeyValue(tt.data)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("formatKeyValue() missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatDashboard(t *testing.T) {
	tests := []struct {
		name     string
		sections []dashboardSection
		want     string
	}{
		{
			name: "multi-section dashboard",
			sections: []dashboardSection{
				{
					Title:   "Sprint Status",
					Content: map[string]any{"status": "IN_PROGRESS", "elapsed": "2h 30m"},
					Format:  "kv",
				},
				{
					Title: "Tasks",
					Content: struct {
						Headers []string
						Rows    [][]string
					}{
						Headers: []string{"ID", "Status"},
						Rows:    [][]string{{"task-1", "IMPLEMENTING_CODE"}},
					},
					Format: "table",
				},
			},
			want: `Sprint Status
=============
status: IN_PROGRESS
elapsed: 2h 30m

Tasks
=====`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDashboard(tt.sections)
			// Just check that it contains section titles
			for _, section := range tt.sections {
				if !strings.Contains(got, section.Title) {
					t.Errorf("formatDashboard() missing section title %q", section.Title)
				}
			}
		})
	}
}
