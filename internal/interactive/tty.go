package interactive

import (
	"os"
)

// IsInteractive returns true if stdin is connected to a terminal.
func IsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
