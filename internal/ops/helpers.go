package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
)

// readTaskState reads state from the blackboard and finds the specified task.
// Returns an error if state cannot be read or the task doesn't exist.
func readTaskState(bb *db.Blackboard, taskID string) (*models.State, *models.Task, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read state: %w", err)
	}
	task := state.FindTask(taskID)
	if task == nil {
		return nil, nil, fmt.Errorf("task not found: %s", taskID)
	}
	return state, task, nil
}
