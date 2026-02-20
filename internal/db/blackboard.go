package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	"github.com/liza-mas/liza/internal/models"
	"gopkg.in/yaml.v3"
)

const (
	DefaultLockTimeout = 10 * time.Second
	LockCheckInterval  = 100 * time.Millisecond
)

// Blackboard provides thread-safe access to the state.yaml file
type Blackboard struct {
	statePath   string
	lockTimeout time.Duration

	// Cache fields for performance optimization
	// We cache raw YAML bytes (not a parsed struct) so that each ReadCached
	// call returns a fresh *models.State. This prevents callers from silently
	// corrupting a shared cached struct.
	cacheMu     sync.RWMutex
	cachedData  []byte
	cachedMtime time.Time

	// Metrics collection (optional)
	metricsRecorder *lockMetricsRecorder
	enableMetrics   bool
}

// New creates a Blackboard backed by the given state file path.
func New(statePath string) *Blackboard {
	return &Blackboard{
		statePath:   statePath,
		lockTimeout: DefaultLockTimeout,
	}
}

// WithLockTimeout creates a new instance; cached bytes are copied (not shared).
func (bb *Blackboard) WithLockTimeout(timeout time.Duration) *Blackboard {
	bb.cacheMu.RLock()
	cachedData := bb.cachedData
	cachedMtime := bb.cachedMtime
	bb.cacheMu.RUnlock()

	newBB := &Blackboard{
		statePath:   bb.statePath,
		lockTimeout: timeout,
		cachedData:  cachedData,
		cachedMtime: cachedMtime,
	}
	return newBB
}

// EnableMetrics enables lock metrics collection.
func (bb *Blackboard) EnableMetrics() {
	if bb.metricsRecorder == nil {
		bb.metricsRecorder = newLockMetricsRecorder()
	}
	bb.enableMetrics = true
}

// DisableMetrics disables lock metrics collection.
func (bb *Blackboard) DisableMetrics() {
	bb.enableMetrics = false
}

// GetMetricsRecorder returns the metrics recorder, or nil if not enabled.
func (bb *Blackboard) GetMetricsRecorder() *lockMetricsRecorder {
	return bb.metricsRecorder
}

func (bb *Blackboard) lockPath() string {
	return bb.statePath + ".lock"
}

func (bb *Blackboard) pidPath() string {
	return bb.statePath + ".lock.pid"
}

func (bb *Blackboard) acquireLockWithPID() (*flock.Flock, error) {
	lock := flock.New(bb.lockPath())
	acquired, err := lock.TryLock()
	if err != nil {
		return nil, classifyLockError(err)
	}
	if !acquired {
		return nil, fmt.Errorf("lock not acquired")
	}

	pid := os.Getpid()
	pidData := []byte(strconv.Itoa(pid))
	if err := os.WriteFile(bb.pidPath(), pidData, 0644); err != nil {
		lock.Unlock()
		return nil, classifyLockError(err)
	}

	return lock, nil
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists (Unix-specific)
	// On Unix, this checks process existence without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (bb *Blackboard) isLockStale() (bool, int) {
	pidData, err := os.ReadFile(bb.pidPath())
	if err != nil {
		// No PID file or can't read it - assume not stale
		return false, 0
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		// Invalid PID format - assume not stale
		return false, 0
	}

	return !isProcessAlive(pid), pid
}

// cleanupStaleLock cleans up after a dead process's lock.
// Only the PID file is removed. The lock file is truncated but preserved
// to maintain inode identity — deleting it would re-introduce the flock
// race described in withLockOperation's defer block.
func (bb *Blackboard) cleanupStaleLock() error {
	os.Remove(bb.pidPath())
	// Truncate lock file (release flock state) without deleting the inode
	if err := os.Truncate(bb.lockPath(), 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to truncate stale lock file: %w", err)
	}
	return nil
}

func (bb *Blackboard) withLock(fn func() error) error {
	return bb.withLockOperation("unknown", fn)
}

func (bb *Blackboard) withLockOperation(operation string, fn func() error) error {
	var lock *flock.Flock
	var err error

	acquisitionStart := time.Now()
	deadline := time.Now().Add(bb.lockTimeout)
	locked := false

	for time.Now().Before(deadline) {
		lock, err = bb.acquireLockWithPID()
		if err == nil {
			locked = true
			break
		}
		// If it's a non-retryable error (permission, disk full, etc.), fail immediately
		if lockErr, ok := err.(*LockError); ok {
			switch lockErr.Type {
			case LockErrorPermission, LockErrorDiskFull, LockErrorFilesystem:
				return lockErr
			}
		}
		time.Sleep(LockCheckInterval)
	}

	if !locked {
		isStale, stalePID := bb.isLockStale()
		if isStale {
			bb.cleanupStaleLock()
			lock, err = bb.acquireLockWithPID()
			if err != nil {
				return newLockStale(stalePID)
			}
			locked = true
		} else {
			return newLockTimeout(fmt.Errorf("lock held by live process after %v", bb.lockTimeout))
		}
	}

	acquisitionTime := time.Since(acquisitionStart)
	holdStart := time.Now()

	// We intentionally do NOT remove the lock file or PID file here.
	// Removing the lock file after unlock creates a race: another process can
	// create a new file (different inode) and acquire flock on it, then this
	// process deletes that file, allowing a third process to create yet another
	// file — resulting in two processes holding flock on different inodes
	// simultaneously. Leaving the file in place ensures all processes flock
	// the same inode. Stale lock cleanup happens only in cleanupStaleLock().
	defer func() {
		lock.Unlock()

		if bb.enableMetrics && bb.metricsRecorder != nil {
			holdTime := time.Since(holdStart)
			bb.metricsRecorder.Record(&lockMetrics{
				Operation:       operation,
				AcquisitionTime: acquisitionTime,
				HoldTime:        holdTime,
			})
		}
	}()

	return fn()
}

// Read returns the current state under an exclusive file lock.
func (bb *Blackboard) Read() (*models.State, error) {
	var state models.State
	err := bb.withLock(func() error {
		data, err := os.ReadFile(bb.statePath)
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("failed to parse state.yaml: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &state, nil
}

// ReadRaw reads the raw state.yaml bytes under flock protection.
// Use this when you need the file content without parsing (e.g., serving
// the raw YAML to an external consumer), while still respecting the lock
// to avoid reading partially-written data.
func (bb *Blackboard) ReadRaw() ([]byte, error) {
	var data []byte
	err := bb.withLock(func() error {
		var readErr error
		data, readErr = os.ReadFile(bb.statePath)
		return readErr
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadCached reads the current state with caching based on file mtime.
// This method avoids disk I/O when the file hasn't changed by caching raw
// YAML bytes. Each call returns a freshly-parsed *models.State, so callers
// can safely mutate the result without corrupting other readers.
func (bb *Blackboard) ReadCached() (*models.State, error) {
	fileInfo, err := os.Stat(bb.statePath)
	if err != nil {
		bb.InvalidateCache()
		return nil, err
	}

	currentMtime := fileInfo.ModTime()

	bb.cacheMu.RLock()
	cachedData := bb.cachedData
	cachedMtime := bb.cachedMtime
	bb.cacheMu.RUnlock()

	var data []byte
	if cachedData != nil && currentMtime.Equal(cachedMtime) {
		data = cachedData
	} else {
		data, err = os.ReadFile(bb.statePath)
		if err != nil {
			bb.InvalidateCache()
			return nil, err
		}

		bb.cacheMu.Lock()
		bb.cachedData = data
		bb.cachedMtime = currentMtime
		bb.cacheMu.Unlock()
	}

	var state models.State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state.yaml: %w", err)
	}

	return &state, nil
}

// InvalidateCache forces the next ReadCached call to reload from disk.
func (bb *Blackboard) InvalidateCache() {
	bb.cacheMu.Lock()
	bb.cachedData = nil
	bb.cachedMtime = time.Time{}
	bb.cacheMu.Unlock()
}

// writeStateData writes data to the state file atomically using fsync + rename.
// Must be called while holding the file lock.
// Uses a unique temp file per call to avoid races if the file lock has gaps.
func (bb *Blackboard) writeStateData(data []byte) error {
	dir := filepath.Dir(bb.statePath)
	base := filepath.Base(bb.statePath)

	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}
	tmpPath := f.Name()

	// CreateTemp uses 0600; match the target file permissions
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set temporary file permissions: %w", err)
	}

	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()

	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write state data: %w", writeErr)
	}
	if syncErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync state file: %w", syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close state file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, bb.statePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Write writes the state to the state file atomically with fsync
func (bb *Blackboard) Write(state *models.State) error {
	err := bb.withLock(func() error {
		data, err := yaml.Marshal(state)
		if err != nil {
			return fmt.Errorf("failed to marshal state: %w", err)
		}
		return bb.writeStateData(data)
	})

	if err == nil {
		bb.InvalidateCache()
	}

	return err
}

// Modify performs an atomic read-modify-write operation
func (bb *Blackboard) Modify(fn func(*models.State) error) error {
	err := bb.withLock(func() error {
		data, err := os.ReadFile(bb.statePath)
		if err != nil {
			return fmt.Errorf("failed to read state: %w", err)
		}

		var state models.State
		if err := yaml.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("failed to parse state: %w", err)
		}

		if err := fn(&state); err != nil {
			return fmt.Errorf("modification function failed: %w", err)
		}

		data, err = yaml.Marshal(&state)
		if err != nil {
			return fmt.Errorf("failed to marshal state: %w", err)
		}
		return bb.writeStateData(data)
	})

	if err == nil {
		bb.InvalidateCache()
	}

	return err
}

// GetTask returns the task with the given ID, or (nil, nil) if not found.
func (bb *Blackboard) GetTask(taskID string) (*models.Task, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, err
	}

	for i := range state.Tasks {
		if state.Tasks[i].ID == taskID {
			return &state.Tasks[i], nil
		}
	}

	return nil, nil
}

// GetAgent returns the agent with the given ID, or (nil, nil) if not found.
func (bb *Blackboard) GetAgent(agentID string) (*models.Agent, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, err
	}

	if agent, ok := state.Agents[agentID]; ok {
		return &agent, nil
	}

	return nil, nil
}

// UpdateTask atomically updates a task by ID
func (bb *Blackboard) UpdateTask(taskID string, fn func(*models.Task) error) error {
	return bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				return fn(&state.Tasks[i])
			}
		}
		return fmt.Errorf("task not found: %s", taskID)
	})
}

// UpdateAgent atomically updates an agent by ID
func (bb *Blackboard) UpdateAgent(agentID string, fn func(*models.Agent) error) error {
	return bb.Modify(func(state *models.State) error {
		agent, ok := state.Agents[agentID]
		if !ok {
			return fmt.Errorf("agent not found: %s", agentID)
		}

		if err := fn(&agent); err != nil {
			return err
		}

		state.Agents[agentID] = agent
		return nil
	})
}

// GetStatePath returns the path to the state file.
func (bb *Blackboard) GetStatePath() string {
	return bb.statePath
}
