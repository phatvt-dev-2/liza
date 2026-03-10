package commands

import (
	"fmt"
	"slices"
	"strings"
)

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
