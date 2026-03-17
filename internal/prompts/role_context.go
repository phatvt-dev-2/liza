package prompts

import "github.com/liza-mas/liza/internal/models"

// RoleContextData is the unified template data type for all role template blocks.
// Each field group is populated as appropriate for the role being rendered.
// Fields not relevant to a particular role remain at their zero value.
type RoleContextData struct {
	// Identity
	Role     string // canonical role name (e.g., "coder", "code-reviewer")
	AgentID  string // agent instance ID (e.g., "coder-1")
	RoleType string // "doer", "reviewer", or "orchestrator"

	// Task (populated for doer and reviewer roles)
	TaskID         string
	Description    string
	DoneWhen       string
	Scope          string
	SpecRef        string
	Worktree       string // resolved absolute path
	IterationNum   int
	AttemptNum     int
	PriorRejection string // empty if no prior rejection

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

	// Declarative (from pipeline YAML)
	MandatoryDocs []string
	Skills        []string
}
