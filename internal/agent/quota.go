package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/paths"
)

// quotaPattern defines a provider-specific pattern that indicates quota exhaustion.
type quotaPattern struct {
	// Provider is the CLIName (e.g. "codex", "claude", "gemini").
	Provider string
	// Needle is a substring to search for in agent output.
	Needle string
}

// quotaPatterns is the registry of known quota-exhaustion signatures.
// Add new entries here when a new provider's quota message is observed.
var quotaPatterns = []quotaPattern{
	{Provider: "codex", Needle: "You've hit your usage limit"},
}

// QuotaExhaustion holds details about a detected quota event.
type QuotaExhaustion struct {
	Provider string
	Message  string // the matching line from output
}

// DetectQuotaExhaustion scans agent output for quota-exhaustion patterns.
// Returns non-nil if a known pattern is found.
func DetectQuotaExhaustion(output, cliName string) *QuotaExhaustion {
	for _, p := range quotaPatterns {
		if p.Provider != cliName {
			continue
		}
		if idx := strings.Index(output, p.Needle); idx != -1 {
			// Extract a reasonable context window around the match.
			start := idx
			end := idx + len(p.Needle) + 100
			if end > len(output) {
				end = len(output)
			}
			// Find end of line.
			if nl := strings.IndexByte(output[start:end], '\n'); nl >= 0 {
				end = start + nl
			}
			return &QuotaExhaustion{
				Provider: p.Provider,
				Message:  output[start:end],
			}
		}
	}
	return nil
}

const quotaSignalPrefix = "provider-quota-exhausted-"

// QuotaSignalPath returns the path to the quota signal file for a provider.
func QuotaSignalPath(projectRoot, provider string) string {
	return filepath.Join(projectRoot, paths.LizaDirName, quotaSignalPrefix+provider)
}

// QuotaSignalGlob returns a glob pattern matching all quota signal files.
func QuotaSignalGlob(projectRoot string) string {
	return filepath.Join(projectRoot, paths.LizaDirName, quotaSignalPrefix+"*")
}

// ProviderFromSignalFile extracts the provider name from a quota signal file path.
func ProviderFromSignalFile(path string) string {
	return filepath.Base(path)[len(quotaSignalPrefix):]
}

// WriteQuotaSignal creates a signal file that tells all supervisors using
// this provider to terminate gracefully.
func WriteQuotaSignal(projectRoot, provider, message string) error {
	signalPath := QuotaSignalPath(projectRoot, provider)
	content := fmt.Sprintf("provider: %s\ndetected: %s\nmessage: %s\n",
		provider,
		time.Now().UTC().Format(time.RFC3339),
		message,
	)
	return os.WriteFile(signalPath, []byte(content), 0644)
}

// CheckQuotaSignal returns true if a quota signal file exists for the provider.
func CheckQuotaSignal(projectRoot, provider string) bool {
	_, err := os.Stat(QuotaSignalPath(projectRoot, provider))
	return err == nil
}

// LogAlert appends an alert line to alerts.log.
func LogAlert(projectRoot, level, category, message string) error {
	alertsPath := filepath.Join(projectRoot, paths.LizaDirName, paths.AlertsLogFileName)
	f, err := os.OpenFile(alertsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open alerts log: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "[%s] %s %s — %s\n",
		time.Now().UTC().Format(time.RFC3339), level, category, message)
	return err
}

// LogQuotaAlert appends a quota-exhaustion alert to alerts.log.
func LogQuotaAlert(projectRoot string, qe *QuotaExhaustion) error {
	return LogAlert(projectRoot, "🚨", "PROVIDER QUOTA EXHAUSTED", qe.Provider+": "+qe.Message)
}

// ClearQuotaSignal removes the quota signal file for a provider.
// Intended for use by `liza resume` or manual recovery.
func ClearQuotaSignal(projectRoot, provider string) error {
	err := os.Remove(QuotaSignalPath(projectRoot, provider))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// tailReadSize is the maximum bytes to read from the end of an output file.
// Quota messages appear near the end; reading the full file is wasteful.
const tailReadSize = 8 * 1024

// latestOutputContent reads the tail of the most recent agent output file.
// Returns empty string if no file is found or read fails.
func latestOutputContent(outputsDir, agentID string) string {
	pattern := filepath.Join(outputsDir, agentID+"-*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	// Glob returns sorted by name; timestamp format ensures lexicographic = chronological.
	latest := matches[len(matches)-1]

	f, err := os.Open(latest)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ""
	}

	size := info.Size()
	readSize := int64(tailReadSize)
	if size < readSize {
		readSize = size
	}
	buf := make([]byte, readSize)
	if _, err := f.ReadAt(buf, size-readSize); err != nil {
		return ""
	}
	return string(buf)
}
