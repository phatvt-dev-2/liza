// Package embedded provides embedded resource files (contracts, skills, specs)
// that are written to project .liza/ directory during initialization.
package embedded

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/paths"
)

// Build-time variables (set via -ldflags during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

//go:embed contracts/*.md
var contractsFS embed.FS

//go:embed skills/*/SKILL.md
var skillsFS embed.FS

//go:embed specs
var specsFS embed.FS

//go:embed "claude-settings.json"
var claudeSettingsContent []byte

//go:embed "mcp.json"
var mcpSettingsContent []byte

// WriteAllFiles writes all embedded files to the project's .liza directory.
// Files are written to contracts/, skills/, and specs/ subdirectories.
// Each file is prepended with YAML frontmatter containing version metadata.
func WriteAllFiles(projectRoot string) error {
	lizaPaths := paths.New(projectRoot)

	// Write contracts
	if err := writeEmbeddedFS(contractsFS, lizaPaths.ContractsDir()); err != nil {
		return fmt.Errorf("failed to write contracts: %w", err)
	}

	// Write skills
	if err := writeEmbeddedFS(skillsFS, lizaPaths.SkillsDir()); err != nil {
		return fmt.Errorf("failed to write skills: %w", err)
	}

	// Write specs
	if err := writeEmbeddedFS(specsFS, lizaPaths.SpecsDir()); err != nil {
		return fmt.Errorf("failed to write specs: %w", err)
	}

	return nil
}

// writeEmbeddedFS writes an entire embedded filesystem to the target directory.
func writeEmbeddedFS(embeddedFS embed.FS, targetDir string) error {
	return fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip root directory
		if path == "." {
			return nil
		}

		// Calculate target path - path from embed includes prefix like "contracts/",
		// but targetDir already points to the contracts directory, so we need to
		// strip the first path component
		// embed.FS always uses forward slashes
		parts := strings.Split(path, "/")
		if len(parts) == 1 {
			// This is the root directory of the embedded FS (e.g., "contracts")
			// Skip it since targetDir already points to this location
			return nil
		}
		// Remove first component (e.g., "contracts/CORE.md" -> "CORE.md")
		relativePath := filepath.Join(parts[1:]...)
		targetPath := filepath.Join(targetDir, relativePath)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, 0755)
		}

		// Read embedded file
		content, err := embeddedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		// Prepend frontmatter
		contentWithFrontmatter := prependFrontmatter(content)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		// Write file
		if err := os.WriteFile(targetPath, contentWithFrontmatter, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}

		return nil
	})
}

// prependFrontmatter adds YAML frontmatter with version metadata to file content.
func prependFrontmatter(content []byte) []byte {
	fm := frontmatter()
	return append([]byte(fm), content...)
}

// frontmatter generates YAML frontmatter string from build-time variables.
func frontmatter() string {
	return fmt.Sprintf(`---
liza_version: "%s"
liza_git_commit: "%s"
liza_build_date: "%s"
---

`, Version, GitCommit, BuildDate)
}

// ListEmbeddedFiles returns a list of all embedded file paths (for testing).
func ListEmbeddedFiles() ([]string, error) {
	var files []string

	// Walk contracts
	err := fs.WalkDir(contractsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path != "." {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Walk skills
	err = fs.WalkDir(skillsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path != "." {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Walk specs
	err = fs.WalkDir(specsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path != "." {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// WriteClaudeSettings writes the embedded claude-settings.json to .claude/settings.json.
// If the file already exists, prompts the user to merge settings.
// Returns nil on success or if user declines merge.
func WriteClaudeSettings(projectRoot string) error {
	lizaPaths := paths.New(projectRoot)
	claudeDir := lizaPaths.ClaudeDir()
	settingsPath := lizaPaths.ClaudeSettingsPath()

	// Check if file already exists
	var existingSettings map[string]any
	if existingData, err := os.ReadFile(settingsPath); err == nil {
		// File exists - prompt user for merge
		fmt.Print("Should the Liza claude settings be merged into the existing settings file? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			// User declined merge - return without error
			return nil
		}

		// Parse existing file
		if err := json.Unmarshal(existingData, &existingSettings); err != nil {
			return fmt.Errorf("failed to parse existing claude-settings.json: %w", err)
		}
	}

	// Create .claude directory if needed
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Parse embedded (liza) settings
	var lizaSettings map[string]any
	if err := json.Unmarshal(claudeSettingsContent, &lizaSettings); err != nil {
		return fmt.Errorf("failed to parse embedded claude-settings.json: %w", err)
	}

	// Merge or use liza settings
	var finalSettings map[string]any
	if existingSettings != nil {
		// Merge: existing settings take precedence
		finalSettings = mergeSettings(lizaSettings, existingSettings)
	} else {
		// New file: use liza settings
		finalSettings = lizaSettings
	}

	// Inject metadata into _comment field
	existingComment, ok := finalSettings["_comment"].([]any)
	if !ok {
		existingComment = []any{}
	}

	metadata := []string{
		"",
		"Generated by liza-go:",
		fmt.Sprintf("  liza_version: %s", Version),
		fmt.Sprintf("  liza_git_commit: %s", GitCommit),
		fmt.Sprintf("  liza_build_date: %s", BuildDate),
	}

	for _, line := range metadata {
		existingComment = append(existingComment, line)
	}
	finalSettings["_comment"] = existingComment

	// Marshal back to JSON with indentation
	output, err := json.MarshalIndent(finalSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal claude-settings.json: %w", err)
	}

	// Write file
	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write claude-settings.json: %w", err)
	}

	return nil
}

// mergeSettings merges liza settings into existing settings.
// Existing settings take precedence (user customizations preserved).
// Special handling for permissions.allow array (union of both).
func mergeSettings(liza, existing map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with liza settings as base
	for k, v := range liza {
		result[k] = v
	}

	// Override with existing settings (preserve user customizations)
	for k, v := range existing {
		if k == "permissions" {
			// Special handling: merge permissions
			lizaPerms, lizaOk := liza[k].(map[string]any)
			existingPerms, existingOk := v.(map[string]any)

			if lizaOk && existingOk {
				result[k] = mergePermissions(lizaPerms, existingPerms)
			} else {
				// If one is invalid, use existing
				result[k] = v
			}
		} else {
			// Direct override
			result[k] = v
		}
	}

	return result
}

// mergePermissions merges permission objects.
// Keeps existing defaultMode, but unions the allow arrays.
func mergePermissions(liza, existing map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with liza permissions
	for k, v := range liza {
		result[k] = v
	}

	// Override with existing (except for "allow" which we union)
	for k, v := range existing {
		if k == "allow" {
			// Union of allow arrays
			lizaAllow, lizaOk := liza[k].([]any)
			existingAllow, existingOk := v.([]any)

			if lizaOk && existingOk {
				result[k] = unionStringArrays(lizaAllow, existingAllow)
			} else if existingOk {
				// If liza doesn't have allow, use existing
				result[k] = v
			}
			// If neither has valid allow, result keeps lizaAllow from earlier copy
		} else {
			// Direct override (e.g., defaultMode)
			result[k] = v
		}
	}

	return result
}

// unionStringArrays returns the union of two string arrays (deduplicated).
func unionStringArrays(a, b []any) []any {
	seen := make(map[string]bool)
	result := []any{}

	for _, item := range a {
		str, ok := item.(string)
		if !ok {
			continue
		}
		if !seen[str] {
			seen[str] = true
			result = append(result, item)
		}
	}

	for _, item := range b {
		str, ok := item.(string)
		if !ok {
			continue
		}
		if !seen[str] {
			seen[str] = true
			result = append(result, item)
		}
	}

	return result
}

// WriteMCPSettings writes the embedded mcp.json to .mcp.json in the project root.
// If the file already exists, prompts the user to merge settings.
// Returns nil on success or if user declines merge.
func WriteMCPSettings(projectRoot string) error {
	mcpSettingsPath := filepath.Join(projectRoot, ".mcp.json")

	// Check if file already exists
	var existingSettings map[string]any
	if existingData, err := os.ReadFile(mcpSettingsPath); err == nil {
		// File exists - prompt user for merge
		fmt.Print("Should the Liza MCP server configuration be merged into the existing .mcp.json file? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			// User declined merge - return without error
			return nil
		}

		// Parse existing file
		if err := json.Unmarshal(existingData, &existingSettings); err != nil {
			return fmt.Errorf("failed to parse existing .mcp.json: %w", err)
		}
	}

	// Parse embedded (liza) settings
	var lizaMCPSettings map[string]any
	if err := json.Unmarshal(mcpSettingsContent, &lizaMCPSettings); err != nil {
		return fmt.Errorf("failed to parse embedded mcp.json: %w", err)
	}

	// Merge or use liza settings
	var finalSettings map[string]any
	if existingSettings != nil {
		// Merge: merge mcpServers maps
		finalSettings = mergeMCPSettings(lizaMCPSettings, existingSettings)
	} else {
		// New file: use liza settings
		finalSettings = lizaMCPSettings
	}

	// Inject metadata into _comment field
	existingComment, ok := finalSettings["_comment"].([]any)
	if !ok {
		existingComment = []any{}
	}

	metadata := []string{
		"",
		"Generated by liza-go:",
		fmt.Sprintf("  liza_version: %s", Version),
		fmt.Sprintf("  liza_git_commit: %s", GitCommit),
		fmt.Sprintf("  liza_build_date: %s", BuildDate),
	}

	for _, line := range metadata {
		existingComment = append(existingComment, line)
	}
	finalSettings["_comment"] = existingComment

	// Marshal back to JSON with indentation
	output, err := json.MarshalIndent(finalSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .mcp.json: %w", err)
	}

	// Write file
	if err := os.WriteFile(mcpSettingsPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	return nil
}

// mergeMCPSettings merges liza MCP settings into existing settings.
// Special handling for mcpServers map (merge server entries).
func mergeMCPSettings(liza, existing map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with liza settings as base
	for k, v := range liza {
		result[k] = v
	}

	// Override with existing settings
	for k, v := range existing {
		if k == "mcpServers" {
			// Special handling: merge mcpServers maps
			lizaServers, lizaOk := liza[k].(map[string]any)
			existingServers, existingOk := v.(map[string]any)

			if lizaOk && existingOk {
				// Merge server entries (existing takes precedence)
				mergedServers := make(map[string]any)
				for serverName, serverConfig := range lizaServers {
					mergedServers[serverName] = serverConfig
				}
				for serverName, serverConfig := range existingServers {
					mergedServers[serverName] = serverConfig
				}
				result[k] = mergedServers
			} else {
				// If one is invalid, use existing
				result[k] = v
			}
		} else {
			// Direct override
			result[k] = v
		}
	}

	return result
}
