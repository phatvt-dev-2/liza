package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/embedded"
)

// userCustomizableFiles are files that users are expected to edit.
// These get individual overwrite prompts even after the bulk confirmation,
// because losing customizations is more costly than re-running setup.
var userCustomizableFiles = map[string]bool{
	"AGENT_TOOLS.md": true,
}

// SetupCommand performs one-time global setup by writing contracts and skills
// to the target directory (typically ~/.liza/).
// The stdin parameter allows for injected input in tests; pass os.Stdin for CLI usage.
func SetupCommand(targetDir string, force bool, stdin io.Reader) error {
	if stdin == nil {
		stdin = os.Stdin
	}
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

	var reader *bufio.Reader

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

		reader = bufio.NewReader(stdin)
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

	// Identify user-customizable files that exist — these need individual confirmation
	skipFiles := make(map[string]bool)
	for _, p := range existing {
		base := filepath.Base(p)
		if userCustomizableFiles[base] {
			if reader == nil {
				reader = bufio.NewReader(stdin)
			}
			fmt.Fprintf(os.Stderr, "%s is user-customizable and has local changes.\n", base)
			fmt.Fprintf(os.Stderr, "Overwrite %s? (y/n): ", relDisplay(targetDir, p))
			response, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read input, skipping %s\n", base)
				skipFiles[p] = true
				continue
			}
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				skipFiles[p] = true
				fmt.Fprintf(os.Stderr, "  Skipped %s (kept existing)\n", base)
			}
		}
	}

	// Backup existing files before overwriting
	for _, p := range existing {
		if skipFiles[p] {
			continue
		}
		if err := backupFile(p); err != nil {
			return fmt.Errorf("failed to backup %s: %w", p, err)
		}
	}

	written, err := embedded.WriteGlobalFiles(targetDir, skipFiles)
	if err != nil {
		return fmt.Errorf("failed to write global files: %w", err)
	}

	if err := embedded.WritePipelineConfig(targetDir); err != nil {
		return fmt.Errorf("failed to write pipeline.yaml: %w", err)
	}

	fmt.Printf("Liza global config written to %s (%d files + pipeline.yaml):\n", targetDir, len(written))
	for _, p := range written {
		fmt.Printf("  %s\n", relDisplay(targetDir, p))
	}
	fmt.Printf("  %s (pipeline config)\n", relDisplay(targetDir, filepath.Join(targetDir, "pipeline.yaml")))
	if len(skipFiles) > 0 {
		fmt.Printf("Skipped %d user-customized files (kept existing).\n", len(skipFiles))
	}
	fmt.Printf("\nNext: configure global permissions in ~/.claude/settings.json\n")
	fmt.Printf("See: contracts/contract-activation.md § Claude\n")
	return nil
}

// backupFile copies src to src.bak, preserving permissions.
func backupFile(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(src + ".bak")
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// relDisplay returns a display path like "~/.liza/CORE.md" using the targetDir as prefix.
func relDisplay(targetDir, path string) string {
	rel, err := filepath.Rel(targetDir, path)
	if err != nil {
		return path
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(targetDir, "/"), rel)
}
