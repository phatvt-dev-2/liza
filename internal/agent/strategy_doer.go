package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// doerStrategy handles task-implementing roles: coder, code-planner, epic-planner, us-writer.
type doerStrategy struct {
	role         string             // runtime role (e.g. "coder")
	workflowRole string             // workflow role (e.g. "coder")
	buildContext contextBuilderFunc // per-role prompt context builder
}

func (s *doerStrategy) DefaultTimeout() time.Duration {
	return 2 * time.Hour
}

func (s *doerStrategy) PreWork(_ context.Context, _ *db.Blackboard, _ SupervisorConfig) (bool, error) {
	return false, nil
}

func (s *doerStrategy) WaitForWork(ctx context.Context, bb *db.Blackboard, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error) {
	pr := loadResolver(config.ProjectRoot)
	return waitForWorkEventDriven(ctx, bb, config.ProjectRoot, pollInterval, maxWait,
		func(state *models.State) (bool, string) {
			claimable := models.CountClaimableTasks(state, s.workflowRole, pr)
			resumableHandoffs := countResumableHandoffTasks(state, config.AgentID, pr)

			logMsg := fmt.Sprintf("%s: %d claimable, %d resumable handoffs", s.role, claimable, resumableHandoffs)

			// Use richer diagnostics for coder role
			if s.role == "coder" {
				logMsg = models.GetCoderWorkDiagnostics(state, pr)
				if resumableHandoffs > 0 {
					handoffMsg := fmt.Sprintf("Found %d resumable handoff task(s) for %s", resumableHandoffs, config.AgentID)
					if logMsg != "" {
						logMsg = handoffMsg + "; " + logMsg
					} else {
						logMsg = handoffMsg
					}
				}
			}

			return claimable > 0 || resumableHandoffs > 0, logMsg
		})
}

func (s *doerStrategy) ClaimTask(config SupervisorConfig, bb *db.Blackboard) (string, string, error) {
	taskID, _, err := claimDoerTask(config.ProjectRoot, config.AgentID, s.workflowRole, bb)
	if err != nil {
		return "", "", err
	}
	return taskID, taskID, nil
}

func (s *doerStrategy) PreExecution(_ *db.Blackboard, _ SupervisorConfig) error {
	return nil
}

func (s *doerStrategy) BuildPrompt(state *models.State, config SupervisorConfig, taskID string) (string, error) {
	return buildPromptWithContext(state, config, taskID, s.buildContext)
}

func (s *doerStrategy) PostExecution(bb *db.Blackboard, config SupervisorConfig, _ string, claimedTaskID string, _ *models.State) error {
	if claimedTaskID == "" {
		return nil
	}

	pr, err := ops.LoadResolverForModels(config.ProjectRoot)
	if err != nil {
		GetLogger().Warn("Failed to load pipeline resolver — skipping submission log", "error", err)
		return nil
	}

	if err := logTaskSubmissionIfCompleted(bb, claimedTaskID, config.AgentID, pr); err != nil {
		GetLogger().Warn("Failed to log task submission", "error", err, "task_id", claimedTaskID)
	}
	return nil
}
