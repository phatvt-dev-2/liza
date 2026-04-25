package projectdetect

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// nodeLockfiles maps lockfiles to their install commands, ordered by specificity.
var nodeLockfiles = []struct {
	file string
	cmd  string
}{
	{"pnpm-lock.yaml", "pnpm install"},
	{"yarn.lock", "yarn install"},
	{"bun.lockb", "bun install"},
	{"bun.lock", "bun install"},
	{"package-lock.json", "npm install"},
}

// DetectInstallCmdInDir checks a single directory for package.json and returns
// the appropriate install command based on which lockfile is present.
// Returns "" if no package.json is found.
func DetectInstallCmdInDir(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
		return ""
	}
	for _, lf := range nodeLockfiles {
		if _, err := os.Stat(filepath.Join(dir, lf.file)); err == nil {
			return lf.cmd
		}
	}
	return "npm install"
}

// DetectLockfileInDir returns the lockfile name found in dir, or "".
func DetectLockfileInDir(dir string) string {
	for _, lf := range nodeLockfiles {
		if _, err := os.Stat(filepath.Join(dir, lf.file)); err == nil {
			return lf.file
		}
	}
	return ""
}

// DetectNodeSubdirs returns sorted subdirectory names (depth 1) containing
// package.json. Dotfile-prefixed directories and node_modules are skipped —
// these commonly contain stray package.json files (build outputs, vendored
// deps) that don't represent real project directories.
func DetectNodeSubdirs(projectRoot string) []string {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		if _, err := os.Stat(filepath.Join(projectRoot, name, "package.json")); err == nil {
			dirs = append(dirs, name)
		}
	}
	slices.Sort(dirs)
	return dirs
}

// DetectPostWorktreeCmd checks the project root (and immediate subdirectories
// if nothing at root) for package.json, returning the appropriate install
// command. For multiple subdirectories, returns "" — the caller should print
// a manual-configuration message instead of guessing at a compound command.
func DetectPostWorktreeCmd(projectRoot string) string {
	if cmd := DetectInstallCmdInDir(projectRoot); cmd != "" {
		return cmd
	}
	subdirs := DetectNodeSubdirs(projectRoot)
	if len(subdirs) == 1 {
		cmd := DetectInstallCmdInDir(filepath.Join(projectRoot, subdirs[0]))
		return "cd " + subdirs[0] + " && " + cmd
	}
	return ""
}

// DetectPkgManagerContext returns a human-readable description of what was
// detected (e.g. "package.json + yarn.lock") for the suggestion prompt.
func DetectPkgManagerContext(projectRoot string) string {
	if _, err := os.Stat(filepath.Join(projectRoot, "package.json")); err == nil {
		if lf := DetectLockfileInDir(projectRoot); lf != "" {
			return "package.json + " + lf
		}
		return "package.json"
	}
	subdirs := DetectNodeSubdirs(projectRoot)
	if len(subdirs) == 1 {
		dir := subdirs[0]
		if lf := DetectLockfileInDir(filepath.Join(projectRoot, dir)); lf != "" {
			return dir + "/package.json + " + dir + "/" + lf
		}
		return dir + "/package.json"
	}
	return "package.json"
}
