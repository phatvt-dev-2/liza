package render

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
			got, err := FormatJSON(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("FormatJSON() =\n%s\n\nwant:\n%s", got, tt.want)
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
			got, err := FormatYAML(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("FormatYAML() =\n%s\n\nwant:\n%s", got, tt.want)
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
			got, err := FormatValue(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FormatValue() = %q, want %q", got, tt.want)
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
			got := FormatTable(tt.headers, tt.rows)
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("FormatTable() =\n%s\n\nwant:\n%s", got, tt.want)
			}
		})
	}
}
