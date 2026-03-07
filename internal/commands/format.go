package commands

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func formatJSON(data any) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

func formatYAML(data any) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return strings.TrimSpace(string(bytes)), nil
}

func formatValue(data any) (string, error) {
	if data == nil {
		return "", nil
	}

	switch v := data.(type) {
	case string:
		return v, nil
	case int, int64, int32, int16, int8:
		return fmt.Sprintf("%d", v), nil
	case uint, uint64, uint32, uint16, uint8:
		return fmt.Sprintf("%d", v), nil
	case float64, float32:
		return fmt.Sprintf("%v", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func formatKeyValue(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	var lines []string
	for _, key := range keys {
		value := data[key]
		if value == nil {
			lines = append(lines, fmt.Sprintf("%s: <none>", key))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %v", key, value))
		}
	}
	return strings.Join(lines, "\n")
}

func formatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Format header
	var lines []string
	headerLine := ""
	for i, header := range headers {
		if i > 0 {
			headerLine += "  "
		}
		headerLine += header + strings.Repeat(" ", widths[i]-len(header))
	}
	lines = append(lines, headerLine)

	// Format rows
	for _, row := range rows {
		rowLine := ""
		for i, cell := range row {
			if i > 0 {
				rowLine += "  "
			}
			if i < len(widths) {
				rowLine += cell + strings.Repeat(" ", widths[i]-len(cell))
			} else {
				rowLine += cell
			}
		}
		lines = append(lines, strings.TrimRight(rowLine, " "))
	}

	return strings.Join(lines, "\n")
}

// dashboardSection represents one section of a dashboard
type dashboardSection struct {
	Title   string
	Content any    // Can be map[string]any or table data
	Format  string // "kv", "table", or "text"
}

func formatDashboard(sections []dashboardSection) string {
	var output []string

	for _, section := range sections {
		// Add section title
		output = append(output, section.Title)
		output = append(output, strings.Repeat("=", len(section.Title)))

		// Format section content based on type
		switch section.Format {
		case "kv":
			if data, ok := section.Content.(map[string]any); ok {
				output = append(output, formatKeyValue(data))
			}
		case "table":
			// Table format expects Content to have Headers and Rows fields
			// This is a simplified version - can be extended
			output = append(output, "")
		case "text":
			output = append(output, fmt.Sprintf("%v", section.Content))
		}

		output = append(output, "")
	}

	return strings.Join(output, "\n")
}
