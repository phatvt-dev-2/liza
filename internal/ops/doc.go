// Package ops implements core task workflow operations as pure business logic.
// Functions return structured results with no terminal I/O side effects.
//
// This package serves as the shared service layer between the CLI command
// handlers (internal/commands) and the agent supervisor (internal/agent).
// Both consume these functions — commands add presentation, agent uses
// structured results for orchestration decisions.
package ops
