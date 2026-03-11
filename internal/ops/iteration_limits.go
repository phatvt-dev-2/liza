package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
)

type limitEscalation struct {
	reason    string
	questions []string
}

func effectiveCoderIterationLimit(task *models.Task, config models.Config) int {
	if task != nil && task.MaxIterations > 0 {
		return task.MaxIterations
	}
	if config.MaxCoderIterations > 0 {
		return config.MaxCoderIterations
	}
	return models.DefaultMaxCoderIterations
}

func effectiveReviewCycleLimit(config models.Config) int {
	if config.MaxReviewCycles > 0 {
		return config.MaxReviewCycles
	}
	return models.DefaultMaxReviewCycles
}

func classifyLimitEscalation(reviewCycles, reviewLimit, iteration, iterationLimit int) (limitEscalation, bool) {
	reviewLimitReached := reviewCycles >= reviewLimit
	iterationLimitReached := iteration >= iterationLimit
	if !reviewLimitReached && !iterationLimitReached {
		return limitEscalation{}, false
	}

	switch {
	case reviewLimitReached && iterationLimitReached:
		return limitEscalation{
			reason:    combinedLimitBlockedReason(reviewCycles, reviewLimit, iteration, iterationLimit),
			questions: defaultCombinedLimitBlockedQuestions(),
		}, true
	case reviewLimitReached:
		return limitEscalation{
			reason:    reviewBudgetExhaustedReason(reviewCycles, reviewLimit),
			questions: defaultReviewBudgetExhaustedQuestions(),
		}, true
	default:
		return limitEscalation{
			reason:    iterationLimitBlockedReason(iteration, iterationLimit),
			questions: defaultIterationLimitBlockedQuestions(),
		}, true
	}
}

func iterationLimitBlockedReason(iteration, limit int) string {
	return fmt.Sprintf("max iterations reached without approval (%d/%d)", iteration, limit)
}

func reviewBudgetExhaustedReason(reviewCycles, limit int) string {
	return fmt.Sprintf("review budget exhausted: max review cycles reached (%d/%d)", reviewCycles, limit)
}

func combinedLimitBlockedReason(reviewCycles, reviewLimit, iteration, iterationLimit int) string {
	return fmt.Sprintf(
		"review budget and iteration limits exhausted (review cycles %d/%d, iterations %d/%d)",
		reviewCycles, reviewLimit, iteration, iterationLimit,
	)
}

func defaultIterationLimitBlockedQuestions() []string {
	return []string{
		"Should this task be rescoped or superseded now that max iterations were exhausted?",
	}
}

func defaultReviewBudgetExhaustedQuestions() []string {
	return []string{
		"Should orchestrator rescope this task or clarify acceptance criteria to break the review cycle?",
	}
}

func defaultCombinedLimitBlockedQuestions() []string {
	return []string{
		"Should orchestrator rescope this task now that both review cycles and iterations are exhausted?",
	}
}
