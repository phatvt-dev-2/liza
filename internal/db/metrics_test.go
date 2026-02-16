package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// TestlockMetrics tests basic metrics structure
func TestLockMetrics(t *testing.T) {
	metrics := &lockMetrics{
		Operation:       "test-operation",
		AcquisitionTime: 50 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	}

	if metrics.Operation != "test-operation" {
		t.Errorf("Operation = %q, want %q", metrics.Operation, "test-operation")
	}

	if metrics.AcquisitionTime != 50*time.Millisecond {
		t.Errorf("AcquisitionTime = %v, want %v", metrics.AcquisitionTime, 50*time.Millisecond)
	}

	if metrics.HoldTime != 100*time.Millisecond {
		t.Errorf("HoldTime = %v, want %v", metrics.HoldTime, 100*time.Millisecond)
	}
}

// TestlockMetricsString tests string representation
func TestLockMetricsString(t *testing.T) {
	metrics := &lockMetrics{
		Operation:       "read",
		AcquisitionTime: 25 * time.Millisecond,
		HoldTime:        75 * time.Millisecond,
	}

	str := metrics.String()

	// Check that the string contains key information
	if !strings.Contains(str, "read") {
		t.Errorf("String() should contain operation name 'read'")
	}

	if !strings.Contains(str, "25ms") {
		t.Errorf("String() should contain acquisition time '25ms'")
	}

	if !strings.Contains(str, "75ms") {
		t.Errorf("String() should contain hold time '75ms'")
	}
}

// TestRecordlockMetrics tests metrics recording
func TestRecordlockMetrics(t *testing.T) {
	recorder := newLockMetricsRecorder()

	metrics1 := &lockMetrics{
		Operation:       "read",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        50 * time.Millisecond,
	}

	metrics2 := &lockMetrics{
		Operation:       "write",
		AcquisitionTime: 20 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	}

	recorder.Record(metrics1)
	recorder.Record(metrics2)

	recorded := recorder.GetMetrics()
	if len(recorded) != 2 {
		t.Errorf("GetMetrics() returned %d metrics, want 2", len(recorded))
	}

	if recorded[0].Operation != "read" {
		t.Errorf("First metric operation = %q, want %q", recorded[0].Operation, "read")
	}

	if recorded[1].Operation != "write" {
		t.Errorf("Second metric operation = %q, want %q", recorded[1].Operation, "write")
	}
}

// TestlockMetricsRecorderClear tests clearing metrics
func TestLockMetricsRecorderClear(t *testing.T) {
	recorder := newLockMetricsRecorder()

	recorder.Record(&lockMetrics{
		Operation:       "test",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        20 * time.Millisecond,
	})

	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("Expected 1 metric before clear")
	}

	recorder.Clear()

	if len(recorder.GetMetrics()) != 0 {
		t.Errorf("Expected 0 metrics after clear")
	}
}

// TestlockMetricsRecorderStats tests aggregate statistics
func TestLockMetricsRecorderStats(t *testing.T) {
	recorder := newLockMetricsRecorder()

	recorder.Record(&lockMetrics{
		Operation:       "op1",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        50 * time.Millisecond,
	})

	recorder.Record(&lockMetrics{
		Operation:       "op2",
		AcquisitionTime: 30 * time.Millisecond,
		HoldTime:        150 * time.Millisecond,
	})

	recorder.Record(&lockMetrics{
		Operation:       "op3",
		AcquisitionTime: 20 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	})

	stats := recorder.GetStats()

	if stats.Count != 3 {
		t.Errorf("Stats.Count = %d, want 3", stats.Count)
	}

	// Average acquisition time: (10 + 30 + 20) / 3 = 20ms
	expectedAvgAcq := 20 * time.Millisecond
	if stats.AvgAcquisitionTime != expectedAvgAcq {
		t.Errorf("Stats.AvgAcquisitionTime = %v, want %v", stats.AvgAcquisitionTime, expectedAvgAcq)
	}

	// Average hold time: (50 + 150 + 100) / 3 = 100ms
	expectedAvgHold := 100 * time.Millisecond
	if stats.AvgHoldTime != expectedAvgHold {
		t.Errorf("Stats.AvgHoldTime = %v, want %v", stats.AvgHoldTime, expectedAvgHold)
	}

	// Max acquisition time: 30ms
	if stats.MaxAcquisitionTime != 30*time.Millisecond {
		t.Errorf("Stats.MaxAcquisitionTime = %v, want %v", stats.MaxAcquisitionTime, 30*time.Millisecond)
	}

	// Max hold time: 150ms
	if stats.MaxHoldTime != 150*time.Millisecond {
		t.Errorf("Stats.MaxHoldTime = %v, want %v", stats.MaxHoldTime, 150*time.Millisecond)
	}
}

// TestlockMetricsRecorderStatsEmpty tests stats with no metrics
func TestLockMetricsRecorderStatsEmpty(t *testing.T) {
	recorder := newLockMetricsRecorder()
	stats := recorder.GetStats()

	if stats.Count != 0 {
		t.Errorf("Stats.Count = %d, want 0", stats.Count)
	}

	if stats.AvgAcquisitionTime != 0 {
		t.Errorf("Stats.AvgAcquisitionTime = %v, want 0", stats.AvgAcquisitionTime)
	}

	if stats.AvgHoldTime != 0 {
		t.Errorf("Stats.AvgHoldTime = %v, want 0", stats.AvgHoldTime)
	}
}

// TestMeasureLockOperation tests measuring a lock operation
func TestMeasureLockOperation(t *testing.T) {
	// Simulate lock operation with delays
	acquireStart := time.Now()
	time.Sleep(10 * time.Millisecond) // Simulate acquisition delay
	acquireEnd := time.Now()

	holdStart := acquireEnd
	time.Sleep(20 * time.Millisecond) // Simulate operation
	holdEnd := time.Now()

	metrics := &lockMetrics{
		Operation:       "test-op",
		AcquisitionTime: acquireEnd.Sub(acquireStart),
		HoldTime:        holdEnd.Sub(holdStart),
	}

	// Allow for some timing variance
	if metrics.AcquisitionTime < 8*time.Millisecond || metrics.AcquisitionTime > 15*time.Millisecond {
		t.Errorf("AcquisitionTime = %v, expected ~10ms", metrics.AcquisitionTime)
	}

	if metrics.HoldTime < 18*time.Millisecond || metrics.HoldTime > 25*time.Millisecond {
		t.Errorf("HoldTime = %v, expected ~20ms", metrics.HoldTime)
	}
}

// TestlockMetricsRecorderConcurrent tests concurrent recording
func TestLockMetricsRecorderConcurrent(t *testing.T) {
	recorder := newLockMetricsRecorder()

	const numGoroutines = 10
	const metricsPerGoroutine = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < metricsPerGoroutine; j++ {
				recorder.Record(&lockMetrics{
					Operation:       "concurrent-op",
					AcquisitionTime: time.Duration(id) * time.Millisecond,
					HoldTime:        time.Duration(j) * time.Millisecond,
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	metrics := recorder.GetMetrics()
	expectedCount := numGoroutines * metricsPerGoroutine
	if len(metrics) != expectedCount {
		t.Errorf("GetMetrics() returned %d metrics, want %d", len(metrics), expectedCount)
	}
}

// TestBlackboardMetricsIntegration tests metrics collection with Blackboard operations
func TestBlackboardMetricsIntegration(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	// Create initial state
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal:    models.Goal{ID: "goal-1", Status: models.GoalStatusInProgress, Created: now},
		Tasks:   []models.Task{},
		Agents:  make(map[string]models.Agent),
		Config:  models.Config{IntegrationBranch: "main"},
	}

	bb := New(statePath)
	bb.EnableMetrics()

	// Write state
	if err := bb.Write(state); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read state
	_, err := bb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check that metrics were collected
	recorder := bb.GetMetricsRecorder()
	if recorder == nil {
		t.Fatal("Metrics recorder is nil")
	}

	metrics := recorder.GetMetrics()
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics (write + read), got %d", len(metrics))
	}

	// Verify metrics have reasonable values
	for _, m := range metrics {
		if m.AcquisitionTime < 0 {
			t.Errorf("Negative acquisition time: %v", m.AcquisitionTime)
		}
		if m.HoldTime < 0 {
			t.Errorf("Negative hold time: %v", m.HoldTime)
		}
	}
}

// TestBlackboardMetricsDisabled tests that metrics are not collected when disabled
func TestBlackboardMetricsDisabled(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	// Create initial state
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal:    models.Goal{ID: "goal-1", Status: models.GoalStatusInProgress, Created: now},
		Tasks:   []models.Task{},
		Agents:  make(map[string]models.Agent),
		Config:  models.Config{IntegrationBranch: "main"},
	}

	bb := New(statePath)
	// Don't enable metrics

	// Write state
	if err := bb.Write(state); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read state
	_, err := bb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Check that no metrics were collected
	recorder := bb.GetMetricsRecorder()
	if recorder != nil {
		metrics := recorder.GetMetrics()
		if len(metrics) > 0 {
			t.Errorf("Expected no metrics when disabled, got %d", len(metrics))
		}
	}
}

// TestBlackboardMetricsToggle tests enabling and disabling metrics
func TestBlackboardMetricsToggle(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.yaml")

	// Create initial state
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal:    models.Goal{ID: "goal-1", Status: models.GoalStatusInProgress, Created: now},
		Tasks:   []models.Task{},
		Agents:  make(map[string]models.Agent),
		Config:  models.Config{IntegrationBranch: "main"},
	}

	bb := New(statePath)

	// Enable metrics and perform operation
	bb.EnableMetrics()
	if err := bb.Write(state); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	recorder := bb.GetMetricsRecorder()
	if recorder == nil {
		t.Fatal("Metrics recorder is nil after enabling")
	}

	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("Expected 1 metric after write, got %d", len(recorder.GetMetrics()))
	}

	// Disable metrics and perform operation
	bb.DisableMetrics()
	if err := bb.Write(state); err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	// Should still have only 1 metric (from before disabling)
	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("Expected 1 metric after disabling, got %d", len(recorder.GetMetrics()))
	}

	// Re-enable and perform operation
	bb.EnableMetrics()
	if err := bb.Write(state); err != nil {
		t.Fatalf("Third write failed: %v", err)
	}

	// Should now have 2 metrics
	if len(recorder.GetMetrics()) != 2 {
		t.Errorf("Expected 2 metrics after re-enabling, got %d", len(recorder.GetMetrics()))
	}
}
