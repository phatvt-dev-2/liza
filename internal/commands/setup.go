package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/embedded"
)

// SetupCommand performs one-time global setup by writing contracts and skills
// to the target directory (typically ~/.liza/).
func SetupCommand(targetDir string, force bool) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	planned := embedded.PlanGlobalFiles(targetDir)

	// Partition into new vs existing files
	var existing, fresh []string
	for _, p := range planned {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		} else {
			fresh = append(fresh, p)
		}
	}

	// If files already exist, require --force and confirmation
	if len(existing) > 0 {
		if !force {
			return fmt.Errorf("global config already exists at %s (%d files), use --force to overwrite",
				targetDir, len(existing))
		}

		fmt.Printf("%d existing files will be overwritten:\n", len(existing))
		for _, p := range existing {
			fmt.Printf("  %s\n", relDisplay(targetDir, p))
		}
		if len(fresh) > 0 {
			fmt.Printf("%d new files will be added.\n", len(fresh))
		}
		fmt.Printf("\nOverwrite? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input, aborting")
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("aborted by user")
		}
		fmt.Println()
	}

	written, err := embedded.WriteGlobalFiles(targetDir)
	if err != nil {
		return fmt.Errorf("failed to write global files: %w", err)
	}

	fmt.Printf("Liza global config written to %s (%d files):\n", targetDir, len(written))
	for _, p := range written {
		fmt.Printf("  %s\n", relDisplay(targetDir, p))
	}
	fmt.Printf("\nNext: configure global permissions in ~/.claude/settings.json\n")
	fmt.Printf("See: contracts/contract-activation.md § Claude\n")
	return nil
}

// relDisplay returns a display path like "~/.liza/CORE.md" using the targetDir as prefix.
func relDisplay(targetDir, path string) string {
	rel, err := filepath.Rel(targetDir, path)
	if err != nil {
		return path
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(targetDir, "/"), rel)
}
