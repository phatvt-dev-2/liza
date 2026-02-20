// Package log provides structured logging to log.yaml with atomic file operations.
package log

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/filelock"
	"gopkg.in/yaml.v3"
)

// Entry represents a single log entry
type Entry struct {
	Timestamp time.Time `yaml:"timestamp"`
	Agent     string    `yaml:"agent"`
	Action    string    `yaml:"action"`
	Task      *string   `yaml:"task,omitempty"`
	Detail    string    `yaml:"detail"`
}

// Validate checks if the entry has required fields
func (e *Entry) Validate() error {
	if e.Agent == "" {
		return fmt.Errorf("agent is required")
	}
	if e.Action == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}

// Logger provides atomic append operations to log.yaml
type Logger struct {
	logPath  string
	fileLock *filelock.FileLock
}

// New creates a new Logger for the given log file path
func New(logPath string) *Logger {
	return &Logger{
		logPath:  logPath,
		fileLock: filelock.New(logPath),
	}
}

// WithLockTimeout returns a new Logger with a custom lock timeout
func (l *Logger) WithLockTimeout(timeout time.Duration) *Logger {
	return &Logger{
		logPath:  l.logPath,
		fileLock: filelock.New(l.logPath).WithTimeout(timeout),
	}
}

// Append adds a log entry to the log file atomically.
// The timestamp is automatically set to the current UTC time.
func (l *Logger) Append(entry Entry) error {
	// Validate entry
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid entry: %w", err)
	}

	// Set timestamp if not already set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	return l.fileLock.WithLockOperation("append", func() error {
		// Read existing entries
		var entries []Entry

		// Check if file exists
		if _, err := os.Stat(l.logPath); err == nil {
			// File exists, read it
			data, err := os.ReadFile(l.logPath)
			if err != nil {
				return fmt.Errorf("failed to read log file: %w", err)
			}

			// Only unmarshal if file is not empty
			if len(data) > 0 {
				if err := yaml.Unmarshal(data, &entries); err != nil {
					return fmt.Errorf("failed to parse log file: %w", err)
				}
			}
		} else if !os.IsNotExist(err) {
			// Some other error
			return fmt.Errorf("failed to stat log file: %w", err)
		}
		// If file doesn't exist, entries will be empty slice

		// Append new entry
		entries = append(entries, entry)

		// Marshal to YAML
		data, err := yaml.Marshal(entries)
		if err != nil {
			return fmt.Errorf("failed to marshal log entries: %w", err)
		}

		// Write atomically
		tmpPath := l.logPath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write temporary log file: %w", err)
		}

		if err := os.Rename(tmpPath, l.logPath); err != nil {
			os.Remove(tmpPath) // Clean up temp file on error
			return fmt.Errorf("failed to rename log file: %w", err)
		}

		return nil
	})
}

// Read reads all entries from the log file.
// This is a convenience method for testing and debugging.
func (l *Logger) Read() ([]Entry, error) {
	var entries []Entry

	data, err := os.ReadFile(l.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	if len(data) == 0 {
		return []Entry{}, nil
	}

	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse log file: %w", err)
	}

	return entries, nil
}
