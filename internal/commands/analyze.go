package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// AnalyzeCommand runs circuit breaker analysis and prints the result to stdout.
// Delegates business logic to ops.Analyze.
func AnalyzeCommand(projectRoot string) error {
	result, err := ops.Analyze(projectRoot)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	if !result.Triggered {
		fmt.Println("Circuit breaker: OK — no patterns detected")
		return nil
	}

	fmt.Println("🚨 CIRCUIT BREAKER TRIGGERED")
	fmt.Printf("Pattern: %s\n", result.Pattern)
	fmt.Printf("Severity: %s\n", result.Severity)
	fmt.Printf("Evidence: %s\n", result.Evidence)
	fmt.Println()
	fmt.Printf("Report written to: %s\n", result.ReportPath)
	return nil
}
