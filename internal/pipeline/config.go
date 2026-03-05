// Package pipeline provides types, parsing, validation, and state resolution
// for declarative pipeline configurations.
package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineConfig is the top-level structure of a pipeline YAML file.
type PipelineConfig struct {
	Pipeline Pipeline `yaml:"pipeline"`
}

// Pipeline defines agent roles, role-pairs, sub-pipelines, and entry-points.
type Pipeline struct {
	AgentRoles   map[string]string      `yaml:"agent-roles"`
	RolePairs    map[string]RolePairDef `yaml:"role-pairs"`
	SubPipelines map[string]SubPipeline `yaml:"sub-pipelines"`
	EntryPoints  map[string]string      `yaml:"entry-points"`
}

// RolePairDef defines a doer-reviewer pair and its state names.
type RolePairDef struct {
	Doer     string         `yaml:"doer"`
	Reviewer string         `yaml:"reviewer"`
	States   RolePairStates `yaml:"states"`
}

// RolePairStates holds the six state names for a role-pair's lifecycle.
type RolePairStates struct {
	Initial   string `yaml:"initial"`
	Executing string `yaml:"executing"`
	Submitted string `yaml:"submitted"`
	Reviewing string `yaml:"reviewing"`
	Approved  string `yaml:"approved"`
	Rejected  string `yaml:"rejected"`
}

// SubPipeline defines an ordered sequence of role-pair steps and transitions between them.
type SubPipeline struct {
	Steps       []string        `yaml:"steps"`
	Transitions []TransitionDef `yaml:"transitions"`
}

// TransitionDef describes a cross-pair transition within a sub-pipeline.
type TransitionDef struct {
	Name        string `yaml:"name"`
	From        string `yaml:"from"`        // e.g., "code-planning-pair.approved"
	To          string `yaml:"to"`          // e.g., "coding-pair.initial"
	Trigger     string `yaml:"trigger"`     // "manual" or "auto"
	Cardinality string `yaml:"cardinality"` // "per-subtask" or "one-to-one"
}

// LoadFrozen loads the frozen pipeline config from .liza/pipeline.yaml.
// Returns nil, nil when no pipeline config exists (legacy goal).
func LoadFrozen(projectRoot string) (*PipelineConfig, error) {
	path := filepath.Join(projectRoot, ".liza", "pipeline.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return Load(path)
}

// Load parses and validates a pipeline config from the given path.
func Load(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline config: %w", err)
	}
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing pipeline config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating pipeline config: %w", err)
	}
	return &cfg, nil
}

// validate checks a PipelineConfig for structural correctness.
func validate(cfg *PipelineConfig) error {
	p := &cfg.Pipeline

	if len(p.RolePairs) == 0 {
		return fmt.Errorf("pipeline must define at least one role-pair")
	}

	// Validate doer/reviewer reference agent-roles.
	for name, rp := range p.RolePairs {
		if _, ok := p.AgentRoles[rp.Doer]; !ok {
			return fmt.Errorf("role-pair %q: doer %q not found in agent-roles", name, rp.Doer)
		}
		if _, ok := p.AgentRoles[rp.Reviewer]; !ok {
			return fmt.Errorf("role-pair %q: reviewer %q not found in agent-roles", name, rp.Reviewer)
		}
	}

	// Validate state names: non-empty and globally unique.
	stateOwner := make(map[string]string) // state name → role-pair name
	for name, rp := range p.RolePairs {
		states := []struct {
			phase string
			value string
		}{
			{"initial", rp.States.Initial},
			{"executing", rp.States.Executing},
			{"submitted", rp.States.Submitted},
			{"reviewing", rp.States.Reviewing},
			{"approved", rp.States.Approved},
			{"rejected", rp.States.Rejected},
		}
		for _, s := range states {
			if s.value == "" {
				return fmt.Errorf("role-pair %q: %s state is empty", name, s.phase)
			}
			if owner, exists := stateOwner[s.value]; exists {
				return fmt.Errorf("duplicate state name %q: used by role-pairs %q and %q", s.value, owner, name)
			}
			stateOwner[s.value] = name
		}
	}

	// Track role-pair membership across sub-pipelines (blocker fix 1).
	rpSubPipeline := make(map[string]string) // role-pair → sub-pipeline name

	// Validate sub-pipelines.
	for spName, sp := range p.SubPipelines {
		if len(sp.Steps) == 0 {
			return fmt.Errorf("sub-pipeline %q: must have at least one step", spName)
		}
		for _, step := range sp.Steps {
			if _, ok := p.RolePairs[step]; !ok {
				return fmt.Errorf("sub-pipeline %q: step %q not found in role-pairs", spName, step)
			}
			// Enforce single-subpipeline membership per role-pair.
			if existingSP, exists := rpSubPipeline[step]; exists {
				return fmt.Errorf("role-pair %q appears in multiple sub-pipelines: %q and %q", step, existingSP, spName)
			}
			rpSubPipeline[step] = spName
		}

		// Validate transitions.
		for _, t := range sp.Transitions {
			if err := validateTransition(t, p, sp.Steps); err != nil {
				return fmt.Errorf("sub-pipeline %q transition %q: %w", spName, t.Name, err)
			}
		}
	}

	// Validate entry-points.
	for epName, epValue := range p.EntryPoints {
		if err := validateEntryPoint(epName, epValue, p); err != nil {
			return err
		}
	}

	return nil
}

// parseRef splits a dotted reference like "role-pair.phase" into its components.
func parseRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid reference %q: expected format <name>.<phase>", ref)
	}
	return parts[0], parts[1], nil
}

// validPhases lists the allowed phase names for role-pair state references.
var validPhases = map[string]bool{
	"initial": true, "executing": true, "submitted": true,
	"reviewing": true, "approved": true, "rejected": true,
}

// validateTransition checks a single transition definition.
func validateTransition(t TransitionDef, p *Pipeline, steps []string) error {
	if t.Name == "" {
		return fmt.Errorf("transition name is empty")
	}

	// Validate trigger.
	if t.Trigger != "manual" && t.Trigger != "auto" {
		return fmt.Errorf("trigger must be %q or %q, got %q", "manual", "auto", t.Trigger)
	}

	// Validate cardinality.
	if t.Cardinality != "per-subtask" && t.Cardinality != "one-to-one" {
		return fmt.Errorf("cardinality must be %q or %q, got %q", "per-subtask", "one-to-one", t.Cardinality)
	}

	// Validate from reference.
	fromPair, fromPhase, err := parseRef(t.From)
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}
	if _, ok := p.RolePairs[fromPair]; !ok {
		return fmt.Errorf("from: role-pair %q not found", fromPair)
	}
	if !validPhases[fromPhase] {
		return fmt.Errorf("from: invalid phase %q", fromPhase)
	}

	// Validate to reference.
	toPair, toPhase, err := parseRef(t.To)
	if err != nil {
		return fmt.Errorf("to: %w", err)
	}
	if _, ok := p.RolePairs[toPair]; !ok {
		return fmt.Errorf("to: role-pair %q not found", toPair)
	}
	if !validPhases[toPhase] {
		return fmt.Errorf("to: invalid phase %q", toPhase)
	}

	// Validate both role-pairs are steps of this sub-pipeline.
	fromInSteps := false
	toInSteps := false
	for _, s := range steps {
		if s == fromPair {
			fromInSteps = true
		}
		if s == toPair {
			toInSteps = true
		}
	}
	if !fromInSteps {
		return fmt.Errorf("from role-pair %q is not a step of this sub-pipeline", fromPair)
	}
	if !toInSteps {
		return fmt.Errorf("to role-pair %q is not a step of this sub-pipeline", toPair)
	}

	return nil
}

// validateEntryPoint validates a single entry-point definition (blocker fix 2).
func validateEntryPoint(epName, epValue string, p *Pipeline) error {
	spName, rpName, err := parseRef(epValue)
	if err != nil {
		return fmt.Errorf("entry-point %q: %w", epName, err)
	}

	sp, ok := p.SubPipelines[spName]
	if !ok {
		return fmt.Errorf("entry-point %q: sub-pipeline %q not found", epName, spName)
	}

	// Validate that the role-pair is a step of the referenced sub-pipeline.
	if !slices.Contains(sp.Steps, rpName) {
		return fmt.Errorf("entry-point %q: role-pair %q is not a step of sub-pipeline %q", epName, rpName, spName)
	}

	return nil
}
