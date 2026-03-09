package models

// State represents the complete Liza state.yaml structure
type State struct {
	Version         int                    `yaml:"version"`
	PipelineVersion int                    `yaml:"pipeline_version,omitempty"`
	Goal            Goal                   `yaml:"goal"`
	Tasks           []Task                 `yaml:"tasks"`
	Agents          map[string]Agent       `yaml:"agents"`
	Discovered      []Discovery            `yaml:"discovered"`
	Handoff         map[string]HandoffNote `yaml:"handoff"`
	HumanNotes      []HumanNote            `yaml:"human_notes"`
	SpecChanges     []SpecChange           `yaml:"spec_changes"`
	Anomalies       []Anomaly              `yaml:"anomalies"`
	Sprint          Sprint                 `yaml:"sprint"`
	SprintHistory   []SprintSummary        `yaml:"sprint_history,omitempty"`
	CircuitBreaker  CircuitBreaker         `yaml:"circuit_breaker"`
	Config          Config                 `yaml:"config"`
	Extra           map[string]any         `yaml:",inline"`
}

// FindTask returns a pointer to the task with the given ID, or nil if not found.
// The returned pointer refers to the element within s.Tasks, so mutations are
// reflected in the state (useful inside Blackboard.Modify closures).
func (s *State) FindTask(taskID string) *Task {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			return &s.Tasks[i]
		}
	}
	return nil
}

// FindTaskIndex returns the index of the task with the given ID, or -1 if not found.
// Use when you need to remove a task from the slice.
func (s *State) FindTaskIndex(taskID string) int {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			return i
		}
	}
	return -1
}
