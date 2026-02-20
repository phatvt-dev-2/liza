// Package filelock provides file-based mutual exclusion using flock(2).
//
// It supports timeout-based acquisition, PID-based stale lock detection,
// classified error types, and optional lock metrics collection.
//
// This package is used by both the blackboard (state.yaml) and the
// structured logger (log.yaml) to serialize concurrent file access.
package filelock
