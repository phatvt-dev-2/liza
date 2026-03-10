package commands

import (
	"strings"
	"testing"
)

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "simple object",
			data: map[string]any{
				"id":     "task-1",
				"status": "IMPLEMENTING_CODE",
			},
			want: `{
  "id": "task-1",
  "status": "IMPLEMENTING_CODE"
}`,
			wantErr: false,
		},
		{
			name:    "string value",
			data:    "RUNNING",
			want:    `"RUNNING"`,
			wantErr: false,
		},
		{
			name:    "number value",
			data:    42,
			want:    `42`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatJSON(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("formatJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("formatJSON() =\n%s\n\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatYAML(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name: "simple object",
			data: map[string]any{
				"id":     "task-1",
				"status": "IMPLEMENTING_CODE",
			},
			want: `id: task-1
status: IMPLEMENTING_CODE`,
			wantErr: false,
		},
		{
			name:    "string value",
			data:    "RUNNING",
			want:    `RUNNING`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatYAML(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("formatYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("formatYAML() =\n%s\n\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    string
		wantErr bool
	}{
		{
			name:    "string",
			data:    "RUNNING",
			want:    "RUNNING",
			wantErr: false,
		},
		{
			name:    "number",
			data:    42,
			want:    "42",
			wantErr: false,
		},
		{
			name:    "boolean",
			data:    true,
			want:    "true",
			wantErr: false,
		},
		{
			name:    "nil",
			data:    nil,
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatValue(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("formatValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

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

func TestFormatTable(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
	}{
		{
			name:    "simple table",
			headers: []string{"ID", "Status", "Count"},
			rows: [][]string{
				{"task-1", "IMPLEMENTING_CODE", "1"},
				{"task-2", "CODE_READY_FOR_REVIEW", "2"},
			},
			want: `ID      Status                 Count
task-1  IMPLEMENTING_CODE      1
task-2  CODE_READY_FOR_REVIEW  2`,
		},
		{
			name:    "empty table",
			headers: []string{"ID", "Status"},
			rows:    [][]string{},
			want:    `ID  Status`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTable(tt.headers, tt.rows)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("formatTable() =\n%s\n\nwant:\n%s", got, tt.want)
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
