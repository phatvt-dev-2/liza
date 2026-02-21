// Package db implements the atomic YAML database layer with file locking.
// It provides safe concurrent access to state.yaml using flock-based mutual exclusion.
//
// # Instance Management
//
// Production code should use [For] to obtain a process-level singleton
// Blackboard for a given state path. This ensures all callers in the same
// process share cache state, preventing silent fragmentation if Blackboard
// gains in-process state (metrics, write batching, subscriptions) in the
// future.
//
// [New] creates an independent instance and is intended for tests that need
// isolation. Each test uses a unique temp directory, so [For] would also
// provide natural isolation, but [New] makes the independence explicit.
package db
