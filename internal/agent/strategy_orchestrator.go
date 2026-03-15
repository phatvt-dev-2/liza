package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// orchestratorStrategy handles the orchestrator role.
type orchestratorStrategy struct {
	executionTimeout time.Duration // from YAML; 0 = use type default
	yamlPollSec      int           // from YAML; 0 = use type default
	yamlMaxWaitSec   int           // from YAML; 0 = use type default
}

const defaultOrchestratorTimeout = 4 * time.Hour

func (s *orchestratorStrategy) DefaultTimeout() time.Duration {
	if s.executionTimeout > 0 {
		return s.executionTimeout
	}
	return defaultOrchestratorTimeout
}

func (s *orchestratorStrategy) WaitConfig(state *models.State) (pollInterval, maxWait time.Duration) {
	poll := nonZeroOr(state.Config.OrchestratorPollInterval, nonZeroOr(s.yamlPollSec, models.DefaultOrchestratorPollInterval))
	max := nonZeroOr(state.Config.OrchestratorMaxWait, nonZeroOr(s.yamlMaxWaitSec, models.DefaultOrchestratorMaxWait))
	return time.Duration(poll) * time.Second, time.Duration(max) * time.Second
}

func (s *orchestratorStrategy) PreWork(_ context.Context, _ *db.Blackboard, _ SupervisorConfig) (bool, error) {
	return false, nil
}

func (s *orchestratorStrategy) WaitForWork(ctx context.Context, bb *db.Blackboard, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error) {
	detCtx, detErr := ops.LoadDetectionContext(config.ProjectRoot)
	var pipelineTerminals []models.TaskStatus
	var planningPairs map[string]bool
	if detErr == nil {
		pipelineTerminals = detCtx.SprintTerminals
		planningPairs = detCtx.PlanningPairs
	}

	return waitForWorkEventDriven(ctx, bb, config.ProjectRoot, pollInterval, maxWait,
		func(state *models.State) (bool, string) {
			result := DetectOrchestratorWakeTriggers(state, pipelineTerminals, planningPairs)
			if result.Trigger != WakeTriggerNone {
				return true, fmt.Sprintf("Orchestrator wake trigger: %s (count: %d)", result.Trigger, result.Count)
			}
			return false, ""
		})
}

func (s *orchestratorStrategy) ClaimTask(_ SupervisorConfig, _ *db.Blackboard) (string, string, error) {
	return "", "", nil
}

func (s *orchestratorStrategy) PreExecution(bb *db.Blackboard, config SupervisorConfig) error {
	return setAgentToOrchestratingStatus(bb, config.AgentID)
}

func (s *orchestratorStrategy) BuildPrompt(state *models.State, config SupervisorConfig, _ string) (string, error) {
	return buildOrchestratorPromptContext(state, config)
}

func (s *orchestratorStrategy) PostExecution(bb *db.Blackboard, config SupervisorConfig, _, _ string, stateBefore *models.State) error {
	detCtx, detErr := ops.LoadDetectionContext(config.ProjectRoot)
	var pipelineTerminals []models.TaskStatus
	var planningPairs map[string]bool
	if detErr != nil {
		GetLogger().Warn("Failed to load detection context", "error", detErr)
	} else {
		pipelineTerminals = detCtx.SprintTerminals
		planningPairs = detCtx.PlanningPairs
	}

	if err := verifyOrchestratorStateChanges(bb, stateBefore, pipelineTerminals, planningPairs); err != nil {
		GetLogger().Warn("Orchestrator state verification failed",
			"error", err,
			"hint", "Agent may not have executed required commands - check prompt file")
	}
	return nil
}
