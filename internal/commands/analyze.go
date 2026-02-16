package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/analysis"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// AnalyzeCommand runs circuit breaker pattern detection analysis
// It reads anomalies from the blackboard, applies pattern rules, and generates a report if triggered
func AnalyzeCommand(projectRoot string) error {
	// Get paths
	lizaPaths := paths.New(projectRoot)
	statePath := lizaPaths.StatePath()
	reportPath := lizaPaths.CircuitBreakerReportPath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Read current state
	state, err := blackboard.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}
	timestamp := time.Now()

	// Detect patterns
	result := analysis.DetectPatterns(state.Anomalies)

	if !result.Triggered {
		// No pattern detected - update status to OK
		err := blackboard.Modify(func(s *models.State) error {
			s.CircuitBreaker.LastCheck = timestamp
			s.CircuitBreaker.Status = "OK"
			s.CircuitBreaker.CurrentTrigger = nil

			// Ensure system mode is RUNNING
			s.Config.Mode = models.SystemModeRunning

			// Add history entry
			s.CircuitBreaker.History = append(s.CircuitBreaker.History, models.CircuitBreakerHistory{
				Timestamp: timestamp,
				Pattern:   nil,
				Result:    "OK",
			})

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to update circuit breaker status: %w", err)
		}

		fmt.Println("Circuit breaker: OK — no patterns detected")
		return nil
	}

	// Pattern triggered
	fmt.Println("🚨 CIRCUIT BREAKER TRIGGERED")
	fmt.Printf("Pattern: %s\n", result.Pattern)
	fmt.Printf("Severity: %s\n", result.Severity)
	fmt.Printf("Evidence: %s\n", result.Evidence)

	// Generate report
	report := analysis.GenerateReport(result, state.Anomalies, timestamp)

	// Write report to file
	err = os.WriteFile(reportPath, []byte(report), 0644)
	if err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	// Update blackboard atomically
	err = blackboard.Modify(func(s *models.State) error {
		s.CircuitBreaker.LastCheck = timestamp
		s.CircuitBreaker.Status = "TRIGGERED"
		s.CircuitBreaker.CurrentTrigger = &models.CircuitBreakerTrigger{
			Timestamp:  timestamp,
			Pattern:    result.Pattern,
			Severity:   result.Severity,
			ReportFile: reportPath,
		}

		// Set system mode to halt agents
		s.Config.Mode = models.SystemModeCircuitBreakerTripped

		// Add history entry
		pattern := result.Pattern
		severity := result.Severity
		s.CircuitBreaker.History = append(s.CircuitBreaker.History, models.CircuitBreakerHistory{
			Timestamp: timestamp,
			Pattern:   &pattern,
			Severity:  &severity,
			Result:    "TRIGGERED",
		})

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update circuit breaker state: %w", err)
	}

	fmt.Println()
	fmt.Printf("Report written to: %s\n", reportPath)

	return nil
}
