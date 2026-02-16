// Package embedded provides embedded resource files (contracts, skills, runtime reference)
// used during workspace initialization.
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

//go:embed "docs/for-agent-eyes/agent-runtime-reference.md"
var runtimeReferenceContent []byte

//go:embed "claude-settings.json"
var claudeSettingsContent []byte

//go:embed "mcp.json"
var mcpSettingsContent []byte

// WriteAllFiles writes embedded files for initialization.
// Contracts and skills are written to .liza/, and the runtime reference
// is written to docs/for-agent-eyes/ in the project root.
// Each file is prepended with YAML frontmatter containing version metadata.
func WriteAllFiles(projectRoot string) error {
	lizaPaths := paths.New(projectRoot)

	if err := writeEmbeddedFS(contractsFS, lizaPaths.ContractsDir()); err != nil {
		return fmt.Errorf("failed to write contracts: %w", err)
	}
	if err := writeEmbeddedFS(skillsFS, lizaPaths.SkillsDir()); err != nil {
		return fmt.Errorf("failed to write skills: %w", err)
	}

	runtimeRefPath := filepath.Join(projectRoot, "docs", "for-agent-eyes", "agent-runtime-reference.md")
	if err := writeEmbeddedFile(runtimeReferenceContent, runtimeRefPath); err != nil {
		return fmt.Errorf("failed to write runtime reference: %w", err)
	}

	return nil
}

// writeEmbeddedFS writes an entire embedded filesystem to the target directory.
func writeEmbeddedFS(embeddedFS embed.FS, targetDir string) error {
	return fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		// embed.FS paths include the top-level dir (e.g. "contracts/CORE.md"),
		// but targetDir already points there — strip the first component.
		parts := strings.Split(path, "/")
		if len(parts) == 1 {
			return nil
		}
		relativePath := filepath.Join(parts[1:]...)
		targetPath := filepath.Join(targetDir, relativePath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, err := embeddedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		contentWithFrontmatter := prependFrontmatter(content)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		if err := os.WriteFile(targetPath, contentWithFrontmatter, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}

		return nil
	})
}

// writeEmbeddedFile writes a single embedded file to the target path.
func writeEmbeddedFile(content []byte, targetPath string) error {
	contentWithFrontmatter := prependFrontmatter(content)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	if err := os.WriteFile(targetPath, contentWithFrontmatter, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}

	return nil
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
	for _, fsys := range []embed.FS{contractsFS, skillsFS} {
		collected, err := collectFiles(fsys)
		if err != nil {
			return nil, err
		}
		files = append(files, collected...)
	}
	files = append(files, "docs/for-agent-eyes/agent-runtime-reference.md")
	return files, nil
}

// collectFiles returns all file paths from an embedded filesystem.
func collectFiles(fsys embed.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path != "." {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// WriteClaudeSettings writes the embedded claude-settings.json to .claude/settings.json.
// If the file already exists, prompts the user to merge settings.
// Returns nil on success or if user declines merge.
func WriteClaudeSettings(projectRoot string) error {
	lizaPaths := paths.New(projectRoot)
	claudeDir := lizaPaths.ClaudeDir()
	settingsPath := lizaPaths.ClaudeSettingsPath()

	var existingSettings map[string]any
	if existingData, err := os.ReadFile(settingsPath); err == nil {
		fmt.Print("Should the Liza claude settings be merged into the existing settings file? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return nil
		}

		if err := json.Unmarshal(existingData, &existingSettings); err != nil {
			return fmt.Errorf("failed to parse existing claude-settings.json: %w", err)
		}
	}

	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	var lizaSettings map[string]any
	if err := json.Unmarshal(claudeSettingsContent, &lizaSettings); err != nil {
		return fmt.Errorf("failed to parse embedded claude-settings.json: %w", err)
	}

	var finalSettings map[string]any
	if existingSettings != nil {
		finalSettings = mergeSettings(lizaSettings, existingSettings)
	} else {
		finalSettings = lizaSettings
	}

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

	output, err := json.MarshalIndent(finalSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal claude-settings.json: %w", err)
	}

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
	for k, v := range liza {
		result[k] = v
	}

	// Existing settings override liza defaults (preserve user customizations),
	// except "permissions" which gets deep-merged to union allow lists.
	for k, v := range existing {
		if k == "permissions" {
			lizaPerms, lizaOk := liza[k].(map[string]any)
			existingPerms, existingOk := v.(map[string]any)
			if lizaOk && existingOk {
				result[k] = mergePermissions(lizaPerms, existingPerms)
			} else {
				result[k] = v
			}
		} else {
			result[k] = v
		}
	}

	return result
}

// mergePermissions merges permission objects.
// Existing values override liza defaults, except "allow" which is unioned.
func mergePermissions(liza, existing map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range liza {
		result[k] = v
	}

	for k, v := range existing {
		if k == "allow" {
			lizaAllow, lizaOk := liza[k].([]any)
			existingAllow, existingOk := v.([]any)
			if lizaOk && existingOk {
				result[k] = unionStringArrays(lizaAllow, existingAllow)
			} else if existingOk {
				result[k] = v
			}
		} else {
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

	var existingSettings map[string]any
	if existingData, err := os.ReadFile(mcpSettingsPath); err == nil {
		fmt.Print("Should the Liza MCP server configuration be merged into the existing .mcp.json file? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return nil
		}

		if err := json.Unmarshal(existingData, &existingSettings); err != nil {
			return fmt.Errorf("failed to parse existing .mcp.json: %w", err)
		}
	}

	var lizaMCPSettings map[string]any
	if err := json.Unmarshal(mcpSettingsContent, &lizaMCPSettings); err != nil {
		return fmt.Errorf("failed to parse embedded mcp.json: %w", err)
	}

	var finalSettings map[string]any
	if existingSettings != nil {
		finalSettings = mergeMCPSettings(lizaMCPSettings, existingSettings)
	} else {
		finalSettings = lizaMCPSettings
	}

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

	output, err := json.MarshalIndent(finalSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .mcp.json: %w", err)
	}

	if err := os.WriteFile(mcpSettingsPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	return nil
}

// mergeMCPSettings merges liza MCP settings into existing settings.
// Existing values override liza defaults, except "mcpServers" which is deep-merged
// (individual server entries merged, existing takes precedence per server name).
func mergeMCPSettings(liza, existing map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range liza {
		result[k] = v
	}

	for k, v := range existing {
		if k == "mcpServers" {
			lizaServers, lizaOk := liza[k].(map[string]any)
			existingServers, existingOk := v.(map[string]any)
			if lizaOk && existingOk {
				mergedServers := make(map[string]any)
				for name, cfg := range lizaServers {
					mergedServers[name] = cfg
				}
				for name, cfg := range existingServers {
					mergedServers[name] = cfg
				}
				result[k] = mergedServers
			} else {
				result[k] = v
			}
		} else {
			result[k] = v
		}
	}

	return result
}
