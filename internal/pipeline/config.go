// Package pipeline provides types, parsing, validation, and state resolution
// for declarative pipeline configurations.
package pipeline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound is returned when no pipeline config file exists.
// Callers can use errors.Is to distinguish absent config (legacy project)
// from parse/validation errors.
var ErrConfigNotFound = errors.New("pipeline config not found")

// PipelineConfig is the top-level structure of a pipeline YAML file.
type PipelineConfig struct {
	Pipeline Pipeline `yaml:"pipeline"`
}

// Pipeline defines roles, role-pairs, sub-pipelines, and entry-points.
type Pipeline struct {
	Roles               map[string]RoleDef     `yaml:"roles,omitempty"`
	RolePairs           map[string]RolePairDef `yaml:"role-pairs"`
	SubPipelines        map[string]SubPipeline `yaml:"sub-pipelines"`
	PipelineTransitions []TransitionDef        `yaml:"pipeline-transitions"`
	EntryPoints         map[string]string      `yaml:"entry-points"`
}

// RoleDef defines a role's properties declaratively.
type RoleDef struct {
	Type              string      `yaml:"type"`
	DisplayName       string      `yaml:"display-name"`
	Description       string      `yaml:"description,omitempty"`
	MaxInstances      int         `yaml:"max-instances,omitempty"`
	Timeouts          *TimeoutDef `yaml:"timeouts,omitempty"`
	ContextSections   []string    `yaml:"context-sections,omitempty"`
	AllowedOperations []string    `yaml:"allowed-operations,omitempty"`
	Skills            []string    `yaml:"skills,omitempty"`
	MandatoryDocs     []string    `yaml:"mandatory-docs,omitempty"`
}

// TimeoutDef holds duration strings for role-specific timeouts.
type TimeoutDef struct {
	Execution    string `yaml:"execution,omitempty"`
	PollInterval string `yaml:"poll-interval,omitempty"`
	MaxWait      string `yaml:"max-wait,omitempty"`
}

// validRoleTypes lists the allowed values for RoleDef.Type.
var validRoleTypes = map[string]bool{
	"doer":         true,
	"reviewer":     true,
	"orchestrator": true,
}

// ReviewPolicyOverrideDef defines quorum overrides for a specific impact level.
type ReviewPolicyOverrideDef struct {
	Quorum            int    `yaml:"quorum"`
	ProviderDiversity string `yaml:"provider-diversity,omitempty"`
}

// ReviewPolicyDef defines the review quorum policy for a role-pair.
type ReviewPolicyDef struct {
	Quorum             int                      `yaml:"quorum"`
	SignificantChange  *ReviewPolicyOverrideDef `yaml:"significant-change,omitempty"`
	ArchitectureImpact *ReviewPolicyOverrideDef `yaml:"architecture-impact,omitempty"`
}

// validProviderDiversity lists the allowed values for provider-diversity.
var validProviderDiversity = map[string]bool{
	"":          true, // omitted / not configured
	"preferred": true,
}

// RolePairDef defines a doer-reviewer pair and its state names.
type RolePairDef struct {
	Doer         string           `yaml:"doer"`
	Reviewer     string           `yaml:"reviewer"`
	ReviewPolicy *ReviewPolicyDef `yaml:"review-policy,omitempty"`
	States       RolePairStates   `yaml:"states"`
}

// RolePairStates holds the six state names for a role-pair's lifecycle.
type RolePairStates struct {
	Initial           string `yaml:"initial"`
	Executing         string `yaml:"executing"`
	Submitted         string `yaml:"submitted"`
	Reviewing         string `yaml:"reviewing"`
	Approved          string `yaml:"approved"`
	Rejected          string `yaml:"rejected"`
	PartiallyApproved string `yaml:"partially-approved,omitempty"`
	Reviewing2        string `yaml:"reviewing-2,omitempty"`
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
// Returns ErrConfigNotFound (wrapped) when no pipeline config exists.
// Callers can use errors.Is(err, ErrConfigNotFound) to distinguish absent
// config from parse/validation errors.
func LoadFrozen(projectRoot string) (*PipelineConfig, error) {
	path := filepath.Join(projectRoot, ".liza", "pipeline.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w at %s (run 'liza init' to create workspace)", ErrConfigNotFound, path)
	}
	return Load(path)
}

// LoadFromBytes parses and validates a pipeline config from raw YAML bytes.
func LoadFromBytes(data []byte) (*PipelineConfig, error) {
	var cfg PipelineConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing pipeline config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating pipeline config: %w", err)
	}
	return &cfg, nil
}

// Load parses and validates a pipeline config from the given path.
func Load(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline config: %w", err)
	}
	var cfg PipelineConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
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

	// Validate roles section if present.
	for name, role := range p.Roles {
		if role.Type == "" {
			return fmt.Errorf("role %q: type field is required", name)
		}
		if !validRoleTypes[role.Type] {
			return fmt.Errorf("role %q: invalid type %q (must be doer, reviewer, or orchestrator)", name, role.Type)
		}
	}

	for name, rp := range p.RolePairs {
		if _, ok := p.Roles[rp.Doer]; !ok {
			return fmt.Errorf("role-pair %q: doer %q not found in roles", name, rp.Doer)
		}
		if _, ok := p.Roles[rp.Reviewer]; !ok {
			return fmt.Errorf("role-pair %q: reviewer %q not found in roles", name, rp.Reviewer)
		}
		if err := validateReviewPolicy(name, rp.ReviewPolicy); err != nil {
			return err
		}
	}

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

	rpSubPipeline := make(map[string]string)   // role-pair → sub-pipeline name
	transitionOwner := make(map[string]string) // transition name → sub-pipeline name

	for spName, sp := range p.SubPipelines {
		if len(sp.Steps) == 0 {
			return fmt.Errorf("sub-pipeline %q: must have at least one step", spName)
		}
		for _, step := range sp.Steps {
			if _, ok := p.RolePairs[step]; !ok {
				return fmt.Errorf("sub-pipeline %q: step %q not found in role-pairs", spName, step)
			}
			if existingSP, exists := rpSubPipeline[step]; exists {
				return fmt.Errorf("role-pair %q appears in multiple sub-pipelines: %q and %q", step, existingSP, spName)
			}
			rpSubPipeline[step] = spName
		}

		for _, t := range sp.Transitions {
			if err := validateTransition(t, p, sp.Steps); err != nil {
				return fmt.Errorf("sub-pipeline %q transition %q: %w", spName, t.Name, err)
			}
			if owner, exists := transitionOwner[t.Name]; exists {
				return fmt.Errorf("duplicate transition name %q: used by sub-pipelines %q and %q", t.Name, owner, spName)
			}
			transitionOwner[t.Name] = spName
		}
	}

	for _, t := range p.PipelineTransitions {
		if err := validatePipelineTransition(t, p); err != nil {
			return fmt.Errorf("pipeline-transition %q: %w", t.Name, err)
		}
		if owner, exists := transitionOwner[t.Name]; exists {
			return fmt.Errorf("duplicate transition name %q: used by %q and pipeline-transitions", t.Name, owner)
		}
		transitionOwner[t.Name] = "pipeline-transitions"
	}

	for epName, epValue := range p.EntryPoints {
		if err := validateEntryPoint(epName, epValue, p); err != nil {
			return err
		}
	}

	return nil
}

// validateReviewPolicy validates a role-pair's review-policy if present.
func validateReviewPolicy(rpName string, rp *ReviewPolicyDef) error {
	if rp == nil {
		return nil
	}
	if rp.Quorum < 1 {
		return fmt.Errorf("role-pair %q: review-policy quorum must be >= 1, got %d", rpName, rp.Quorum)
	}
	if err := validateReviewPolicyOverride(rpName, "significant-change", rp.SignificantChange); err != nil {
		return err
	}
	if err := validateReviewPolicyOverride(rpName, "architecture-impact", rp.ArchitectureImpact); err != nil {
		return err
	}
	return nil
}

// validateReviewPolicyOverride validates a single review-policy override.
func validateReviewPolicyOverride(rpName, overrideName string, override *ReviewPolicyOverrideDef) error {
	if override == nil {
		return nil
	}
	if override.Quorum < 1 {
		return fmt.Errorf("role-pair %q: review-policy %s quorum must be >= 1, got %d", rpName, overrideName, override.Quorum)
	}
	if !validProviderDiversity[override.ProviderDiversity] {
		return fmt.Errorf("role-pair %q: review-policy %s provider-diversity %q is invalid (must be \"preferred\" or omitted)", rpName, overrideName, override.ProviderDiversity)
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

// parse3PartRef splits a dotted reference like "sub-pipeline.role-pair.phase" into its components.
// Used for pipeline-transitions that reference states across sub-pipeline boundaries.
func parse3PartRef(ref string) (subPipeline, rolePair, phase string, err error) {
	parts := strings.SplitN(ref, ".", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid 3-part reference %q: expected format <sub-pipeline>.<role-pair>.<phase>", ref)
	}
	return parts[0], parts[1], parts[2], nil
}

// validPhases lists the allowed phase names for role-pair state references.
var validPhases = map[string]bool{
	"initial": true, "executing": true, "submitted": true,
	"reviewing": true, "approved": true, "rejected": true,
}

// validateTransitionHeader checks the common fields shared by all transition types:
// non-empty name, valid trigger, and valid cardinality.
func validateTransitionHeader(t TransitionDef) error {
	if t.Name == "" {
		return fmt.Errorf("transition name is empty")
	}
	if t.Trigger != "manual" && t.Trigger != "auto" {
		return fmt.Errorf("trigger must be %q or %q, got %q", "manual", "auto", t.Trigger)
	}
	if t.Cardinality != "per-subtask" && t.Cardinality != "one-to-one" {
		return fmt.Errorf("cardinality must be %q or %q, got %q", "per-subtask", "one-to-one", t.Cardinality)
	}
	return nil
}

// validateTransition checks a single transition definition.
func validateTransition(t TransitionDef, p *Pipeline, steps []string) error {
	if err := validateTransitionHeader(t); err != nil {
		return err
	}

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

	if !slices.Contains(steps, fromPair) {
		return fmt.Errorf("from role-pair %q is not a step of this sub-pipeline", fromPair)
	}
	if !slices.Contains(steps, toPair) {
		return fmt.Errorf("to role-pair %q is not a step of this sub-pipeline", toPair)
	}

	return nil
}

// validatePipelineTransition checks a single pipeline-transition definition.
// Pipeline-transitions use 3-part refs and must reference different sub-pipelines.
func validatePipelineTransition(t TransitionDef, p *Pipeline) error {
	if err := validateTransitionHeader(t); err != nil {
		return err
	}

	fromSP, fromPair, fromPhase, err := parse3PartRef(t.From)
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}
	sp, ok := p.SubPipelines[fromSP]
	if !ok {
		return fmt.Errorf("from: sub-pipeline %q not found", fromSP)
	}
	if !slices.Contains(sp.Steps, fromPair) {
		return fmt.Errorf("from: role-pair %q is not a step of sub-pipeline %q", fromPair, fromSP)
	}
	if !validPhases[fromPhase] {
		return fmt.Errorf("from: invalid phase %q", fromPhase)
	}

	toSP, toPair, toPhase, err := parse3PartRef(t.To)
	if err != nil {
		return fmt.Errorf("to: %w", err)
	}
	sp, ok = p.SubPipelines[toSP]
	if !ok {
		return fmt.Errorf("to: sub-pipeline %q not found", toSP)
	}
	if !slices.Contains(sp.Steps, toPair) {
		return fmt.Errorf("to: role-pair %q is not a step of sub-pipeline %q", toPair, toSP)
	}
	if !validPhases[toPhase] {
		return fmt.Errorf("to: invalid phase %q", toPhase)
	}

	if fromSP == toSP {
		return fmt.Errorf("from and to must reference different sub-pipelines (both reference %q)", fromSP)
	}

	return nil
}

// validateEntryPoint validates a single entry-point definition.
func validateEntryPoint(epName, epValue string, p *Pipeline) error {
	spName, rpName, err := parseRef(epValue)
	if err != nil {
		return fmt.Errorf("entry-point %q: %w", epName, err)
	}

	sp, ok := p.SubPipelines[spName]
	if !ok {
		return fmt.Errorf("entry-point %q: sub-pipeline %q not found", epName, spName)
	}

	if !slices.Contains(sp.Steps, rpName) {
		return fmt.Errorf("entry-point %q: role-pair %q is not a step of sub-pipeline %q", epName, rpName, spName)
	}

	return nil
}
