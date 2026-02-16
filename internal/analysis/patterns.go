package analysis

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"gopkg.in/yaml.v3"
)

// PatternResult represents the result of pattern detection
type PatternResult struct {
	Triggered bool
	Pattern   string
	Severity  string
	Evidence  string
}

// DetectPatterns analyzes anomalies and detects circuit breaker patterns
// Returns the first matching pattern, or a non-triggered result if none match
func DetectPatterns(anomalies []models.Anomaly) PatternResult {
	// Check patterns in order (as per bash implementation)

	// 1. retry_cluster: 3+ retry_loops with similar error_pattern
	if result := checkRetryCluster(anomalies); result.Triggered {
		return result
	}

	// 2. debt_accumulation: 3+ trade_offs with debt_created=true
	if result := checkDebtAccumulation(anomalies); result.Triggered {
		return result
	}

	// 3. assumption_cascade: 2+ assumption_violated with same assumption
	if result := checkAssumptionCascade(anomalies); result.Triggered {
		return result
	}

	// 4. spec_gap_cluster: 2+ spec_ambiguity with same spec_ref
	if result := checkSpecGapCluster(anomalies); result.Triggered {
		return result
	}

	// 5. workaround_pattern: 2+ workarounds/trade_offs with similar root_cause
	if result := checkWorkaroundPattern(anomalies); result.Triggered {
		return result
	}

	// 6. external_service_outage: 2+ external_blockers with same blocker_service
	if result := checkExternalServiceOutage(anomalies); result.Triggered {
		return result
	}

	return PatternResult{Triggered: false}
}

// checkRetryCluster detects retry_cluster pattern
func checkRetryCluster(anomalies []models.Anomaly) PatternResult {
	retryLoops := filterByType(anomalies, "retry_loop")
	if len(retryLoops) < 3 {
		return PatternResult{Triggered: false}
	}

	// Count similar error patterns
	groups := groupByField(retryLoops, "error_pattern")
	for _, group := range groups {
		if len(group) >= 2 {
			// At least 2 anomalies share the same error pattern
			return PatternResult{
				Triggered: true,
				Pattern:   "retry_cluster",
				Severity:  "ARCHITECTURE_FLAW",
				Evidence:  fmt.Sprintf("%d retry_loop anomalies with similar error patterns", len(retryLoops)),
			}
		}
	}

	return PatternResult{Triggered: false}
}

// checkDebtAccumulation detects debt_accumulation pattern
func checkDebtAccumulation(anomalies []models.Anomaly) PatternResult {
	tradeOffs := filterByType(anomalies, "trade_off")

	debtCount := 0
	for _, a := range tradeOffs {
		if debtCreated, ok := a.Details["debt_created"].(bool); ok && debtCreated {
			debtCount++
		}
	}

	if debtCount >= 3 {
		return PatternResult{
			Triggered: true,
			Pattern:   "debt_accumulation",
			Severity:  "SCOPE_FLAW",
			Evidence:  fmt.Sprintf("%d trade-offs creating technical debt", debtCount),
		}
	}

	return PatternResult{Triggered: false}
}

// checkAssumptionCascade detects assumption_cascade pattern
func checkAssumptionCascade(anomalies []models.Anomaly) PatternResult {
	assumptions := filterByType(anomalies, "assumption_violated")

	groups := groupByField(assumptions, "assumption")
	for _, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "assumption_cascade",
				Severity:  "SPEC_FLAW",
				Evidence:  "Same assumption violated across multiple tasks",
			}
		}
	}

	return PatternResult{Triggered: false}
}

// checkSpecGapCluster detects spec_gap_cluster pattern
func checkSpecGapCluster(anomalies []models.Anomaly) PatternResult {
	specAmbiguities := filterByType(anomalies, "spec_ambiguity")

	groups := groupByField(specAmbiguities, "spec_ref")
	for _, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "spec_gap_cluster",
				Severity:  "SPEC_FLAW",
				Evidence:  "Multiple tasks hitting same spec ambiguity",
			}
		}
	}

	return PatternResult{Triggered: false}
}

// checkWorkaroundPattern detects workaround_pattern
func checkWorkaroundPattern(anomalies []models.Anomaly) PatternResult {
	// Filter workarounds and trade_offs
	workarounds := []models.Anomaly{}
	for _, a := range anomalies {
		if a.Type == "workaround" || a.Type == "trade_off" {
			workarounds = append(workarounds, a)
		}
	}

	if len(workarounds) < 2 {
		return PatternResult{Triggered: false}
	}

	// Group by root_cause (or "what" field as fallback)
	groups := make(map[string][]models.Anomaly)
	for _, a := range workarounds {
		key := ""
		if rootCause, ok := a.Details["root_cause"].(string); ok {
			key = rootCause
		} else if what, ok := a.Details["what"].(string); ok {
			key = what
		}

		if key != "" {
			groups[key] = append(groups[key], a)
		}
	}

	for _, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "workaround_pattern",
				Severity:  "ARCHITECTURE_FLAW",
				Evidence:  fmt.Sprintf("%d workarounds/trade-offs with similar root causes", len(workarounds)),
			}
		}
	}

	return PatternResult{Triggered: false}
}

// checkExternalServiceOutage detects external_service_outage pattern
func checkExternalServiceOutage(anomalies []models.Anomaly) PatternResult {
	externals := filterByType(anomalies, "external_blocker")

	groups := groupByField(externals, "blocker_service")
	for service, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "external_service_outage",
				Severity:  "EXTERNAL_DEPENDENCY",
				Evidence:  fmt.Sprintf("Multiple tasks blocked by same external service: %s", service),
			}
		}
	}

	return PatternResult{Triggered: false}
}

// filterByType returns anomalies of a specific type
func filterByType(anomalies []models.Anomaly, anomalyType string) []models.Anomaly {
	result := []models.Anomaly{}
	for _, a := range anomalies {
		if a.Type == anomalyType {
			result = append(result, a)
		}
	}
	return result
}

// groupByField groups anomalies by a field value in their Details
func groupByField(anomalies []models.Anomaly, field string) map[string][]models.Anomaly {
	groups := make(map[string][]models.Anomaly)
	for _, a := range anomalies {
		if value, ok := a.Details[field].(string); ok && value != "" {
			groups[value] = append(groups[value], a)
		}
	}
	return groups
}

// GenerateReport creates a markdown report for a triggered circuit breaker
func GenerateReport(result PatternResult, anomalies []models.Anomaly, timestamp time.Time) string {
	var sb strings.Builder

	sb.WriteString("# Circuit Breaker Report\n\n")
	sb.WriteString(fmt.Sprintf("**Triggered:** %s\n", timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Pattern:** %s\n", result.Pattern))
	sb.WriteString(fmt.Sprintf("**Severity:** %s\n\n", result.Severity))

	sb.WriteString("## Trigger Evidence\n\n")
	sb.WriteString(result.Evidence)
	sb.WriteString("\n\n")

	sb.WriteString("## Anomalies (raw)\n\n")
	sb.WriteString("```yaml\n")

	// Marshal anomalies to YAML
	yamlData, err := yaml.Marshal(anomalies)
	if err != nil {
		sb.WriteString(fmt.Sprintf("Error marshaling anomalies: %v\n", err))
	} else {
		sb.Write(yamlData)
	}

	sb.WriteString("```\n\n")

	sb.WriteString("## Human Decision Required\n\n")
	sb.WriteString("- [ ] Acknowledge report\n")
	sb.WriteString("- [ ] Confirm severity assessment\n")
	sb.WriteString("- [ ] Determine remediation\n")
	sb.WriteString("- [ ] Release checkpoint with decision logged\n")

	return sb.String()
}
