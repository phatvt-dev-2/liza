// Package embedded provides embedded resource files (contracts, skills, settings)
// used during workspace initialization.
package embedded

import (
	"bufio"
	"bytes"
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

//go:embed skills
var skillsFS embed.FS

//go:embed "claude-settings.json"
var claudeSettingsContent []byte

//go:embed "mcp.json"
var mcpSettingsContent []byte

// PlanGlobalFiles returns the list of absolute paths that WriteGlobalFiles would create,
// without actually writing anything. Useful for pre-flight checks and verbose output.
func PlanGlobalFiles(targetDir string) []string {
	var paths []string
	for _, pair := range []struct {
		fs      embed.FS
		destDir string
	}{
		{contractsFS, targetDir},
		{skillsFS, filepath.Join(targetDir, "skills")},
	} {
		_ = fs.WalkDir(pair.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || path == "." {
				return nil
			}
			parts := strings.Split(path, "/")
			if len(parts) <= 1 {
				return nil
			}
			paths = append(paths, filepath.Join(pair.destDir, filepath.Join(parts[1:]...)))
			return nil
		})
	}
	return paths
}

// WriteGlobalFiles writes contracts and skills to the global Liza directory (~/.liza/).
// Contracts are written flat into targetDir/ and skills into targetDir/skills/.
// Each file is prepended with YAML frontmatter containing version metadata.
// Files whose absolute path appears in skipFiles are silently skipped.
// Returns the list of absolute paths written.
func WriteGlobalFiles(targetDir string, skipFiles map[string]bool) ([]string, error) {
	var written []string
	contractPaths, err := writeEmbeddedFS(contractsFS, targetDir, skipFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to write contracts: %w", err)
	}
	written = append(written, contractPaths...)

	skillPaths, err := writeEmbeddedFS(skillsFS, filepath.Join(targetDir, "skills"), skipFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to write skills: %w", err)
	}
	written = append(written, skillPaths...)

	return written, nil
}

// writeEmbeddedFS writes an entire embedded filesystem to the target directory.
// Files whose absolute path appears in skipFiles are silently skipped.
// Returns the list of absolute paths of files written (directories excluded).
func writeEmbeddedFS(embeddedFS embed.FS, targetDir string, skipFiles map[string]bool) ([]string, error) {
	var written []string
	err := fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
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

		if skipFiles[targetPath] {
			return nil
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

		written = append(written, targetPath)
		return nil
	})
	return written, err
}

// prependFrontmatter merges version metadata into existing YAML frontmatter,
// or prepends a new frontmatter block if none exists.
// Existing non-liza fields (e.g., skill name/description) are preserved;
// old liza_* fields are replaced with current build values.
func prependFrontmatter(content []byte) []byte {
	prefix := []byte("---\n")
	if !bytes.HasPrefix(content, prefix) {
		return append([]byte(frontmatter()), content...)
	}

	rest := content[len(prefix):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx == -1 {
		return append([]byte(frontmatter()), content...) // malformed, prepend fresh
	}

	existingBlock := string(rest[:idx])
	after := rest[idx+len("\n---\n"):]

	// Keep non-liza fields, drop old liza_* fields
	var kept []string
	for _, line := range strings.Split(existingBlock, "\n") {
		if !strings.HasPrefix(line, "liza_") {
			kept = append(kept, line)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	for _, line := range kept {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteString(versionFields())
	buf.WriteString("---\n")

	// Preserve blank line between frontmatter and body
	if len(after) > 0 && after[0] != '\n' {
		buf.WriteByte('\n')
	}
	buf.Write(after)

	return buf.Bytes()
}

// stripFrontmatter removes a leading YAML frontmatter block ("---\n...---\n") if present.
func stripFrontmatter(content []byte) []byte {
	prefix := []byte("---\n")
	if !bytes.HasPrefix(content, prefix) {
		return content
	}
	// Find the closing "---" delimiter
	rest := content[len(prefix):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx == -1 {
		return content // malformed frontmatter, leave as-is
	}
	// Skip past closing "---\n" and any single trailing blank line
	after := rest[idx+len("\n---\n"):]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}
	return after
}

// versionFields returns the liza_* key-value lines without YAML delimiters.
func versionFields() string {
	return fmt.Sprintf("liza_version: \"%s\"\nliza_git_commit: \"%s\"\nliza_build_date: \"%s\"\n",
		Version, GitCommit, BuildDate)
}

// frontmatter generates a complete YAML frontmatter block from build-time variables.
func frontmatter() string {
	return "---\n" + versionFields() + "---\n\n"
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
