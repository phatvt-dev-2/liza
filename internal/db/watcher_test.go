package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"gopkg.in/yaml.v3"
)

// TestStateWatcherBasicNotification tests that watcher sends notifications on file changes
func TestStateWatcherBasicNotification(t *testing.T) {
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
	if err := bb.Write(state); err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Start watcher
	watcher, err := bb.WatchForChanges()
	if err != nil {
		t.Fatalf("WatchForChanges failed: %v", err)
	}
	defer watcher.Close()

	// Modify state
	time.Sleep(10 * time.Millisecond) // Allow watcher to start
	if err := bb.Modify(func(s *models.State) error {
		s.Version = 2
		return nil
	}); err != nil {
		t.Fatalf("Modify failed: %v", err)
	}

	// Wait for notification
	select {
	case <-watcher.Events():
		// Success - notification received
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error from watcher: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}

// TestStateWatcherMultipleWrites tests that multiple rapid writes coalesce
func TestStateWatcherMultipleWrites(t *testing.T) {
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
	if err := bb.Write(state); err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Start watcher
	watcher, err := bb.WatchForChanges()
	if err != nil {
		t.Fatalf("WatchForChanges failed: %v", err)
	}
	defer watcher.Close()

	time.Sleep(10 * time.Millisecond) // Allow watcher to start

	// Perform multiple rapid modifications
	for i := 2; i <= 5; i++ {
		version := i
		if err := bb.Modify(func(s *models.State) error {
			s.Version = version
			return nil
		}); err != nil {
			t.Fatalf("Modify %d failed: %v", i, err)
		}
	}

	// Should receive at least one notification
	// (multiple writes may coalesce into fewer notifications)
	select {
	case <-watcher.Events():
		// Success - at least one notification received
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error from watcher: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}

	// Drain any additional notifications (coalescing may vary by platform)
	drainTimeout := time.After(100 * time.Millisecond)
	notificationCount := 1
drainLoop:
	for {
		select {
		case <-watcher.Events():
			notificationCount++
		case <-drainTimeout:
			break drainLoop
		}
	}

	// We expect fewer notifications than writes due to coalescing
	// but at least 1 notification should be received
	if notificationCount < 1 {
		t.Errorf("Expected at least 1 notification, got %d", notificationCount)
	}
}

// TestStateWatcherClose tests graceful watcher shutdown
func TestStateWatcherClose(t *testing.T) {
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
	if err := bb.Write(state); err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Start watcher
	watcher, err := bb.WatchForChanges()
	if err != nil {
		t.Fatalf("WatchForChanges failed: %v", err)
	}

	// Close watcher
	if err := watcher.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify channels are closed
	select {
	case _, ok := <-watcher.Events():
		if ok {
			t.Error("Events channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Events channel not closed in time")
	}
}

// TestStateWatcherDoubleClose tests that closing twice doesn't panic
func TestStateWatcherDoubleClose(t *testing.T) {
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
	if err := bb.Write(state); err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Start watcher
	watcher, err := bb.WatchForChanges()
	if err != nil {
		t.Fatalf("WatchForChanges failed: %v", err)
	}

	// Close watcher twice - should not panic
	if err := watcher.Close(); err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Second close should be safe (may return error, but shouldn't panic)
	_ = watcher.Close()
}

// TestStateWatcherNonExistentFile tests watcher behavior with non-existent file
func TestStateWatcherNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nonexistent.yaml")

	bb := New(statePath)

	// Watcher should handle non-existent file gracefully
	// It may return an error or watch the directory
	watcher, err := bb.WatchForChanges()
	if err != nil {
		// If error is returned, that's acceptable
		t.Logf("WatchForChanges returned expected error: %v", err)
		return
	}
	defer watcher.Close()

	// If no error, watcher should work once file is created
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal:    models.Goal{ID: "goal-1", Status: models.GoalStatusInProgress, Created: now},
		Tasks:   []models.Task{},
		Agents:  make(map[string]models.Agent),
		Config:  models.Config{IntegrationBranch: "main"},
	}

	if err := bb.Write(state); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should receive notification for file creation
	select {
	case <-watcher.Events():
		// Success - notification received
	case err := <-watcher.Errors():
		t.Logf("Error from watcher (may be expected): %v", err)
	case <-time.After(2 * time.Second):
		t.Log("Timeout waiting for notification (may be expected for new file)")
	}
}

// TestStateWatcherExternalModification tests detection of external changes
func TestStateWatcherExternalModification(t *testing.T) {
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
	if err := bb.Write(state); err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Start watcher
	watcher, err := bb.WatchForChanges()
	if err != nil {
		t.Fatalf("WatchForChanges failed: %v", err)
	}
	defer watcher.Close()

	time.Sleep(10 * time.Millisecond) // Allow watcher to start

	// External modification (direct file write)
	state.Version = 3
	data, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Should receive notification for external change
	select {
	case <-watcher.Events():
		// Success - notification received
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error from watcher: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}
