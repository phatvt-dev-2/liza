package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liza-mas/liza/internal/embedded"
)

// userCustomizableFiles are files that users are expected to edit.
// These get individual overwrite prompts even after the bulk confirmation,
// because losing customizations is more costly than re-running setup.
var userCustomizableFiles = map[string]bool{
	"AGENT_TOOLS.md": true,
}

// SetupParams holds all parameters for the setup command.
type SetupParams struct {
	TargetDir      string    // target directory (typically ~/.liza/)
	Force          bool      // overwrite existing files
	AgentToolsPath string    // path to custom AGENT_TOOLS.md (empty = use embedded)
	Stdin          io.Reader // input for interactive prompts (nil = os.Stdin)
}

// SetupCommand performs one-time global setup by writing contracts and skills
// to the target directory (typically ~/.liza/).
func SetupCommand(params SetupParams) error {
	stdin := params.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	// Early validation: read custom agent-tools file before any filesystem changes.
	var customAgentTools []byte
	if params.AgentToolsPath != "" {
		content, err := os.ReadFile(params.AgentToolsPath)
		if err != nil {
			return fmt.Errorf("failed to read custom agent-tools file %s: %w", params.AgentToolsPath, err)
		}
		customAgentTools = content
	}

	if err := os.MkdirAll(params.TargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", params.TargetDir, err)
	}

	planned := embedded.PlanGlobalFiles(params.TargetDir)
	existing, fresh := partitionByExistence(planned)

	// When --agent-tools is provided, auto-skip the embedded AGENT_TOOLS.md
	// so confirmOverwrites won't prompt for it.
	var autoSkip map[string]bool
	if customAgentTools != nil {
		autoSkip = map[string]bool{
			filepath.Join(params.TargetDir, "AGENT_TOOLS.md"): true,
		}
	}

	skipFiles, err := confirmOverwrites(existing, fresh, params.Force, params.TargetDir, stdin, autoSkip)
	if err != nil {
		return err
	}

	for _, p := range existing {
		if skipFiles[p] {
			continue
		}
		if err := backupFile(p); err != nil {
			return fmt.Errorf("failed to backup %s: %w", p, err)
		}
	}

	written, err := embedded.WriteGlobalFiles(params.TargetDir, skipFiles)
	if err != nil {
		return fmt.Errorf("failed to write global files: %w", err)
	}

	// Write custom AGENT_TOOLS.md, replacing the embedded version.
	var autoReplaced []string
	if customAgentTools != nil {
		agentToolsTarget := filepath.Join(params.TargetDir, "AGENT_TOOLS.md")

		// Back up existing file before overwriting (data-loss protection).
		if _, err := os.Stat(agentToolsTarget); err == nil {
			if err := backupFile(agentToolsTarget); err != nil {
				return fmt.Errorf("failed to backup %s: %w", agentToolsTarget, err)
			}
		}

		contentWithFrontmatter := embedded.PrependFrontmatter(customAgentTools)
		if err := os.WriteFile(agentToolsTarget, contentWithFrontmatter, 0644); err != nil {
			return fmt.Errorf("failed to write custom AGENT_TOOLS.md: %w", err)
		}
		written = append(written, agentToolsTarget)
		autoReplaced = append(autoReplaced, agentToolsTarget)
	}

	if err := embedded.WritePipelineConfig(params.TargetDir); err != nil {
		return fmt.Errorf("failed to write pipeline.yaml: %w", err)
	}

	printSetupSummary(params.TargetDir, written, skipFiles, autoReplaced)
	return nil
}

// partitionByExistence splits paths into those that exist on disk and those that don't.
func partitionByExistence(paths []string) (existing, fresh []string) {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		} else {
			fresh = append(fresh, p)
		}
	}
	return
}

// confirmOverwrites handles interactive confirmation for overwriting existing files.
// Returns the set of files to skip (user-customizable files the user declined to overwrite,
// plus any auto-skipped files from the autoSkip set).
// Files in autoSkip are added to skipFiles without prompting (e.g., when replaced by --agent-tools).
func confirmOverwrites(existing, fresh []string, force bool, targetDir string, stdin io.Reader, autoSkip map[string]bool) (map[string]bool, error) {
	skipFiles := make(map[string]bool)

	// Seed with auto-skipped files (no prompt needed).
	for p := range autoSkip {
		skipFiles[p] = true
	}

	if len(existing) == 0 {
		return skipFiles, nil
	}

	if !force {
		return nil, fmt.Errorf("global config already exists at %s (%d files), use --force to overwrite",
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

	reader := bufio.NewReader(stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return nil, fmt.Errorf("aborted by user")
	}
	fmt.Println()

	// Per-file prompts for user-customizable files (skip if auto-skipped).
	for _, p := range existing {
		if autoSkip[p] {
			continue
		}
		base := filepath.Base(p)
		if !userCustomizableFiles[base] {
			continue
		}
		fmt.Fprintf(os.Stderr, "%s is user-customizable and has local changes.\n", base)
		fmt.Fprintf(os.Stderr, "Overwrite %s? (y/n): ", relDisplay(targetDir, p))
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read input for %s: %v\n", base, err)
			skipFiles[p] = true
			continue
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			skipFiles[p] = true
			fmt.Fprintf(os.Stderr, "  Skipped %s (kept existing)\n", base)
		}
	}

	return skipFiles, nil
}

// printSetupSummary prints the final setup results to stdout.
// autoReplaced tracks files replaced by custom versions (not "kept existing").
func printSetupSummary(targetDir string, written []string, skipFiles map[string]bool, autoReplaced []string) {
	fmt.Printf("Liza global config written to %s (%d files + pipeline.yaml):\n", targetDir, len(written))
	for _, p := range written {
		fmt.Printf("  %s\n", relDisplay(targetDir, p))
	}
	fmt.Printf("  %s (pipeline config)\n", relDisplay(targetDir, filepath.Join(targetDir, "pipeline.yaml")))

	// Count user-skipped files (excluding auto-replaced ones).
	keptExisting := 0
	for p := range skipFiles {
		if !slices.Contains(autoReplaced, p) {
			keptExisting++
		}
	}
	if keptExisting > 0 {
		fmt.Printf("Skipped %d user-customized files (kept existing).\n", keptExisting)
	}
	if len(autoReplaced) == 1 {
		fmt.Printf("Replaced 1 file with custom version.\n")
	} else if len(autoReplaced) > 1 {
		fmt.Printf("Replaced %d files with custom versions.\n", len(autoReplaced))
	}

	fmt.Printf("\nNext: configure global permissions in ~/.claude/settings.json\n")
	fmt.Printf("See: contracts/contract-activation.md § Claude\n")
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
