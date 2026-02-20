package filelock

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileLockBasic(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)

	executed := false
	err := fl.WithLock(func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
	if !executed {
		t.Error("function was not executed")
	}
}

func TestFileLockOperation(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)
	fl.EnableMetrics()

	err := fl.WithLockOperation("test-op", func() error {
		return nil
	})

	if err != nil {
		t.Fatalf("WithLockOperation() error = %v", err)
	}

	recorder := fl.GetMetricsRecorder()
	if recorder == nil {
		t.Fatal("metrics recorder is nil")
	}

	metrics := recorder.GetMetrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Operation != "test-op" {
		t.Errorf("operation = %q, want %q", metrics[0].Operation, "test-op")
	}
}

func TestFileLockWithTimeout(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)
	customFL := fl.WithTimeout(5 * time.Second)

	if customFL.lockTimeout != 5*time.Second {
		t.Errorf("lockTimeout = %v, want %v", customFL.lockTimeout, 5*time.Second)
	}

	// Verify paths are preserved
	if customFL.lockPath != fl.lockPath {
		t.Errorf("lockPath = %q, want %q", customFL.lockPath, fl.lockPath)
	}
}

func TestFileLockConcurrent(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)

	// Write initial data
	if err := os.WriteFile(protectedPath, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	const numGoroutines = 10
	var counter atomic.Int64
	done := make(chan error, numGoroutines)

	for range numGoroutines {
		go func() {
			err := fl.WithLock(func() error {
				counter.Add(1)
				time.Sleep(1 * time.Millisecond) // Simulate work
				return nil
			})
			done <- err
		}()
	}

	for range numGoroutines {
		if err := <-done; err != nil {
			t.Errorf("WithLock() error = %v", err)
		}
	}

	if counter.Load() != numGoroutines {
		t.Errorf("counter = %d, want %d", counter.Load(), numGoroutines)
	}
}

func TestFileLockCreatesLockFile(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)

	err := fl.WithLock(func() error {
		// Lock file should exist during operation
		lockPath := protectedPath + ".lock"
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("lock file does not exist during operation")
		}

		// PID file should exist during operation
		pidPath := protectedPath + ".lock.pid"
		if _, err := os.Stat(pidPath); os.IsNotExist(err) {
			t.Error("PID file does not exist during operation")
		}

		return nil
	})

	if err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
}

func TestFileLockMetricsEnableDisable(t *testing.T) {
	dir := t.TempDir()
	protectedPath := filepath.Join(dir, "data.yaml")

	fl := New(protectedPath)

	// Initially no recorder
	if fl.GetMetricsRecorder() != nil {
		t.Error("expected nil recorder before enabling")
	}

	// Enable
	fl.EnableMetrics()
	if fl.GetMetricsRecorder() == nil {
		t.Error("expected non-nil recorder after enabling")
	}

	// Perform operation
	fl.WithLock(func() error { return nil })

	recorder := fl.GetMetricsRecorder()
	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("expected 1 metric, got %d", len(recorder.GetMetrics()))
	}

	// Disable and perform operation
	fl.DisableMetrics()
	fl.WithLock(func() error { return nil })

	// Should still have only 1 metric
	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("expected 1 metric after disable, got %d", len(recorder.GetMetrics()))
	}

	// Re-enable and perform operation
	fl.EnableMetrics()
	fl.WithLock(func() error { return nil })

	// Should now have 2 metrics
	if len(recorder.GetMetrics()) != 2 {
		t.Errorf("expected 2 metrics after re-enable, got %d", len(recorder.GetMetrics()))
	}
}

func TestFileLockPaths(t *testing.T) {
	fl := New("/tmp/test/state.yaml")

	if fl.lockPath != "/tmp/test/state.yaml.lock" {
		t.Errorf("lockPath = %q, want %q", fl.lockPath, "/tmp/test/state.yaml.lock")
	}
	if fl.pidPath != "/tmp/test/state.yaml.lock.pid" {
		t.Errorf("pidPath = %q, want %q", fl.pidPath, "/tmp/test/state.yaml.lock.pid")
	}
}

func TestFileLockDefaults(t *testing.T) {
	fl := New("/tmp/test")

	if fl.lockTimeout != DefaultLockTimeout {
		t.Errorf("lockTimeout = %v, want %v", fl.lockTimeout, DefaultLockTimeout)
	}
}
