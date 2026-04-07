package prompts

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/models"
)

// MCPToolPrefix returns the MCP tool name prefix for the given CLI provider.
// Claude Code, Gemini, Kimi, and Mistral/Vibe are assumed to follow the
// mcp__<server>__<tool> convention. Codex exposes MCP tools without a prefix.
// Add cases here if a future provider uses a different convention.
func MCPToolPrefix(cliName string) string {
	if cliName == "codex" {
		return ""
	}
	return "mcp__liza__"
}

// toolSearchHint builds a ToolSearch instruction for the given tool prefix and
// comma-separated bare tool names. Returns empty string when prefix is empty
// (provider exposes tools directly without deferred resolution).
func toolSearchHint(prefix, bareTools string) string {
	if prefix == "" {
		return ""
	}
	names := strings.Split(bareTools, ",")
	for i, n := range names {
		names[i] = prefix + strings.TrimSpace(n)
	}
	return fmt.Sprintf(" (resolve AFTER initialization: ToolSearch select:%s)", strings.Join(names, ","))
}

// CompletedTaskSummary provides context about completed tasks for integration analysis.
type CompletedTaskSummary struct {
	ID          string
	Description string
	DoneWhen    string
	SpecRef     string
}

// ParentTaskContext provides context about a parent task for architecture consolidation.
type ParentTaskContext struct {
	ID          string
	Description string
	DoneWhen    string
	SpecRef     string
	PlanRef     string
}

// RoleContextData is the unified template data type for all role template blocks.
// Each field group is populated as appropriate for the role being rendered.
// Fields not relevant to a particular role remain at their zero value.
type RoleContextData struct {
	// Identity
	Role     string // canonical role name (e.g., "coder", "code-reviewer")
	AgentID  string // agent instance ID (e.g., "coder-1")
	RoleType string // "doer", "reviewer", or "orchestrator"

	// Task (populated for doer and reviewer roles)
	TaskID                string
	Description           string
	DoneWhen              string
	Scope                 string
	SpecRef               string
	PlanRef               string // file path only (no fragment)
	PlanSection           string // anchor fragment (e.g., "capability-cap-001---task-creation"), empty if none
	ArchRef               string // path to architecture document, empty if none
	ValidationPlan        string
	Worktree              string // resolved absolute path
	IterationNum          int
	AttemptNum            int
	PriorRejection        string // empty if no prior rejection
	PriorAttemptOutcome   string // reason from prior attempt (empty unless AttemptNum == 2)
	PriorAttemptRejection string // reviewer feedback from prior attempt (empty unless AttemptNum == 2 and Note present)

	// Review (populated for reviewer roles)
	BaseCommit      string // git diff base for reviewer
	ReviewCommit    string // git diff target for reviewer
	AssignedTo      string // code author being reviewed
	ReviewCycles    int
	ScopeExtensions []map[string]string

	// Plan scoping (populated for task-aware roles)
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int // 1-based position in sprint plan
	DependsOn      []string
	TaskRolePair   string

	// Architecture-specific (populated for architect role)
	ParentTaskContexts []ParentTaskContext

	// Integration-specific (populated for integration-analyst and integration-reviewer)
	GoalBaseCommit string
	CompletedTasks []CompletedTaskSummary

	// Coder-specific
	IntegrationBranch string
	IntegrationFix    bool // whether task is in integration fix mode
	HandoffNote       *models.HandoffEvent

	// Orchestrator-specific (pre-rendered content strings)
	DashboardOutput   string
	WakeInstruction   string
	AgentStates       string
	SprintMetrics     string
	ActivePolicies    string
	BlockedTasks      string
	CheckpointSummary string
	PipelineConfig    string

	// Config/state
	ProjectRoot string
	StatePath   string
	SpecsDir    string
	GoalDesc    string
	ToolPrefix  string // MCP tool name prefix — provider-dependent (e.g. "mcp__liza__" for Claude Code, "" for Codex)

	// Declarative (from pipeline YAML)
	MandatoryDocs []string
	Skills        []string
}
