package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

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
