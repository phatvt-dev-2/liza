package render

import (
	"fmt"
	"time"
)

// FormatDuration formats a duration in a human-readable format.
// Examples: "45s", "15m", "2h 30m", "1d 1h 45m"
func FormatDuration(d time.Duration) string {
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
