package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatJSON marshals data as indented JSON.
func FormatJSON(data any) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

// FormatYAML marshals data as YAML with trailing whitespace trimmed.
func FormatYAML(data any) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return strings.TrimSpace(string(bytes)), nil
}

// FormatValue formats a single scalar value as a string.
func FormatValue(data any) (string, error) {
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

// FormatTable renders headers and rows as an aligned text table.
func FormatTable(headers []string, rows [][]string) string {
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
