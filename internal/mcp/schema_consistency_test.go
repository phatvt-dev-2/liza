package mcp

import (
	"slices"
	"testing"

	"github.com/liza-mas/liza/internal/mcp/protocol"
)

// ToolSchemaConsistencyTest verifies that each MCP tool's declared InputSchema
// matches its handler's actual parameter extraction.
//
// This test addresses the schema drift issue where hand-coded InputSchemas
// can diverge from handler implementations (e.g., submit-for-review SHA field regression).
//
// For each tool, it verifies:
// 1. Schema-declared Required fields are actually extracted by the handler
// 2. Handler doesn't require fields not declared in schema
//
// Note: This test uses a parameter map based on code review of handlers.go
// to define expected parameter extraction patterns.

// ParameterExpectation defines expected handler behavior for a parameter
type ParameterExpectation struct {
	Required bool
	Source   string // extraction pattern: "requireString", "requireTaskAndAgent", "extractStringSlice", "direct"
}

// expectedToolParams maps tool names to their expected parameter extraction patterns
// This is derived from manual analysis of handlers.go
var expectedToolParams = map[string]map[string]ParameterExpectation{
	"liza_get": {
		"query":  {Required: true, Source: "requireString"},
		"format": {Required: false, Source: "direct"},
	},
	"liza_status": {},
	"liza_validate": {
		"skip_spec_check": {Required: false, Source: "direct"},
	},
	"liza_version": {},
	"liza_add_task": {
		"id":       {Required: true, Source: "requireString"},
		"desc":     {Required: true, Source: "requireString"},
		"spec":     {Required: true, Source: "requireString"},
		"done":     {Required: true, Source: "requireString"},
		"scope":    {Required: true, Source: "requireString"},
		"agent_id": {Required: false, Source: "direct"},
		"priority": {Required: false, Source: "direct"},
		"depends":  {Required: false, Source: "extractStringSlice"},
		"type":     {Required: false, Source: "direct"},
	},
	"liza_claim_task": {
		"task_id":  {Required: true, Source: "requireTaskAndAgent"},
		"agent_id": {Required: true, Source: "requireTaskAndAgent"},
	},
	"liza_submit_for_review": {
		"task_id":    {Required: true, Source: "requireString"},
		"commit_sha": {Required: true, Source: "requireString"},
		"agent_id":   {Required: true, Source: "requireString"},
	},
	"liza_handoff": {
		"task_id":     {Required: true, Source: "requireTaskAndAgent"},
		"agent_id":    {Required: true, Source: "requireTaskAndAgent"},
		"summary":     {Required: true, Source: "requireString"},
		"next_action": {Required: true, Source: "requireString"},
	},
	"liza_submit_verdict": {
		"task_id":  {Required: true, Source: "requireString"},
		"verdict":  {Required: true, Source: "requireString"},
		"agent_id": {Required: true, Source: "requireString"},
		"reason":   {Required: false, Source: "direct"},
	},
	"liza_mark_blocked": {
		"task_id":   {Required: true, Source: "requireTaskAndAgent"},
		"agent_id":  {Required: true, Source: "requireTaskAndAgent"},
		"reason":    {Required: true, Source: "requireString"},
		"questions": {Required: true, Source: "extractStringSlice"},
	},
	"liza_release_claim": {
		"task_id":  {Required: true, Source: "requireString"},
		"role":     {Required: true, Source: "requireString"},
		"agent_id": {Required: true, Source: "requireString"},
		"reason":   {Required: false, Source: "direct"},
		"force":    {Required: false, Source: "direct"},
	},
	"liza_supersede_task": {
		"task_id":         {Required: true, Source: "requireTaskAndAgent"},
		"agent_id":        {Required: true, Source: "requireTaskAndAgent"},
		"reason":          {Required: true, Source: "requireString"},
		"replacement_ids": {Required: false, Source: "extractStringSlice"},
	},
	"liza_wt_create": {
		"task_id": {Required: true, Source: "requireString"},
		"fresh":   {Required: false, Source: "direct"},
	},
	"liza_wt_delete": {
		"task_id": {Required: true, Source: "requireString"},
	},
	"liza_wt_merge": {
		"task_id":  {Required: true, Source: "requireTaskAndAgent"},
		"agent_id": {Required: true, Source: "requireTaskAndAgent"},
	},
	"liza_analyze":                   {},
	"liza_update_sprint_metrics":     {},
	"liza_checkpoint":                {},
	"liza_clear_stale_review_claims": {},
	"liza_delete_agent": {
		"agent_id": {Required: true, Source: "requireString"},
		"reason":   {Required: true, Source: "requireString"},
		"force":    {Required: false, Source: "direct"},
	},
}

// TestAllToolsSchemaConsistency verifies that schema Required fields match
// the expected parameter extraction patterns defined in expectedToolParams.
func TestAllToolsSchemaConsistency(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	toolNames := server.ToolNames()
	if len(toolNames) == 0 {
		t.Fatal("No tools registered")
	}

	for _, toolName := range toolNames {
		t.Run(toolName, func(t *testing.T) {
			tool, ok := server.GetTool(toolName)
			if !ok {
				t.Fatalf("Tool %s not found", toolName)
			}

			expectedParams, hasExpectations := expectedToolParams[toolName]
			if !hasExpectations {
				t.Fatalf("No parameter expectations defined for tool %s - add to expectedToolParams map", toolName)
			}

			// Extract required fields from schema
			schemaRequired := extractSchemaRequired(tool.InputSchema)

			// Extract required fields from expectations
			expectedRequired := make([]string, 0)
			expectedOptional := make([]string, 0)
			for paramName, exp := range expectedParams {
				if exp.Required {
					expectedRequired = append(expectedRequired, paramName)
				} else {
					expectedOptional = append(expectedOptional, paramName)
				}
			}

			// Check 1: Schema Required fields must be in expected Required
			for _, schemaReq := range schemaRequired {
				found := false
				for _, expReq := range expectedRequired {
					if schemaReq == expReq {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Field %q is declared Required in schema but handler does NOT extract it as required", schemaReq)
				}
			}

			// Check 2: Expected Required fields must be in schema Required
			for _, expReq := range expectedRequired {
				found := false
				for _, schemaReq := range schemaRequired {
					if expReq == schemaReq {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Handler extracts %q as required but schema does NOT declare it as Required", expReq)
				}
			}

			// Check 3: Expected Optional fields must exist in schema Properties
			for _, optField := range expectedOptional {
				if _, ok := tool.InputSchema.Properties[optField]; !ok {
					t.Errorf("Handler extracts optional field %q but it is NOT defined in schema Properties", optField)
				}
			}

			// Check 4: Schema Properties should have expectations defined
			for propName := range tool.InputSchema.Properties {
				if _, ok := expectedParams[propName]; !ok {
					t.Errorf("Schema defines property %q but no expectation defined in test - potential untested parameter", propName)
				}
			}
		})
	}
}

// TestSpecificToolConsistency tests specific tools with known expected patterns
func TestSpecificToolConsistency(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	tests := []struct {
		name           string
		expectRequired []string
		expectOptional []string
	}{
		{
			name:           "liza_get",
			expectRequired: []string{"query"},
			expectOptional: []string{"format"},
		},
		{
			name:           "liza_status",
			expectRequired: nil,
			expectOptional: nil,
		},
		{
			name:           "liza_validate",
			expectRequired: nil,
			expectOptional: []string{"skip_spec_check"},
		},
		{
			name:           "liza_add_task",
			expectRequired: []string{"id", "desc", "spec", "done", "scope"},
			expectOptional: []string{"priority", "depends", "type", "agent_id"},
		},
		{
			name:           "liza_claim_task",
			expectRequired: []string{"task_id", "agent_id"},
			expectOptional: nil,
		},
		{
			name:           "liza_submit_for_review",
			expectRequired: []string{"task_id", "commit_sha", "agent_id"},
			expectOptional: nil,
		},
		{
			name:           "liza_handoff",
			expectRequired: []string{"task_id", "summary", "next_action", "agent_id"},
			expectOptional: nil,
		},
		{
			name:           "liza_submit_verdict",
			expectRequired: []string{"task_id", "verdict", "agent_id"},
			expectOptional: []string{"reason"},
		},
		{
			name:           "liza_mark_blocked",
			expectRequired: []string{"task_id", "agent_id", "reason", "questions"},
			expectOptional: nil,
		},
		{
			name:           "liza_release_claim",
			expectRequired: []string{"task_id", "role", "agent_id"},
			expectOptional: []string{"reason", "force"},
		},
		{
			name:           "liza_supersede_task",
			expectRequired: []string{"task_id", "reason", "agent_id"},
			expectOptional: []string{"replacement_ids"},
		},
		{
			name:           "liza_wt_create",
			expectRequired: []string{"task_id"},
			expectOptional: []string{"fresh"},
		},
		{
			name:           "liza_wt_delete",
			expectRequired: []string{"task_id"},
			expectOptional: nil,
		},
		{
			name:           "liza_wt_merge",
			expectRequired: []string{"task_id", "agent_id"},
			expectOptional: nil,
		},
		{
			name:           "liza_analyze",
			expectRequired: nil,
			expectOptional: nil,
		},
		{
			name:           "liza_update_sprint_metrics",
			expectRequired: nil,
			expectOptional: nil,
		},
		{
			name:           "liza_checkpoint",
			expectRequired: nil,
			expectOptional: nil,
		},
		{
			name:           "liza_clear_stale_review_claims",
			expectRequired: nil,
			expectOptional: nil,
		},
		{
			name:           "liza_delete_agent",
			expectRequired: []string{"agent_id", "reason"},
			expectOptional: []string{"force"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := server.GetTool(tt.name)
			if !ok {
				t.Fatalf("Tool %s not found", tt.name)
			}

			// Verify schema required fields match expectations
			schemaRequired := extractSchemaRequired(tool.InputSchema)

			if !stringSlicesEqual(schemaRequired, tt.expectRequired) {
				t.Errorf("Schema required fields mismatch:\n  got: %v\n  want: %v",
					schemaRequired, tt.expectRequired)
			}

			// Verify optional fields exist in schema properties
			for _, optField := range tt.expectOptional {
				if _, ok := tool.InputSchema.Properties[optField]; !ok {
					t.Errorf("Expected optional field %q not found in schema properties", optField)
				}
			}

			// Verify required fields are NOT in optional list
			for _, reqField := range tt.expectRequired {
				if slices.Contains(tt.expectOptional, reqField) {
					t.Errorf("Field %q appears in both required and optional lists", reqField)
				}
			}
		})
	}
}

// TestSchemaDriftRegression ensures the specific submit-for-review SHA field issue is caught
// This test will FAIL if commit_sha is missing from either schema or handler expectations
func TestSchemaDriftRegression(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	tool, ok := server.GetTool("liza_submit_for_review")
	if !ok {
		t.Fatal("liza_submit_for_review tool not found")
	}

	// The regression was: schema declared commit_sha as required,
	// but handler didn't extract it properly (or vice versa)
	required := extractSchemaRequired(tool.InputSchema)

	// commit_sha must be required
	if !slices.Contains(required, "commit_sha") {
		t.Error("commit_sha is not declared as required in schema - this would cause a regression")
	}

	// Handler must require commit_sha (from expectedToolParams)
	params, ok := expectedToolParams["liza_submit_for_review"]
	if !ok {
		t.Fatal("No expectations defined for liza_submit_for_review")
	}

	commitSHAExpectation, ok := params["commit_sha"]
	if !ok {
		t.Fatal("commit_sha expectation not defined - this would cause a regression")
	}

	if !commitSHAExpectation.Required {
		t.Error("Handler expectation does not mark commit_sha as required - this would cause a regression")
	}
}

// TestToolRegistrationCompleteness verifies all ~20 tools are registered
func TestToolRegistrationCompleteness(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	// Expected tools count (approximately 20 as mentioned in issue)
	expectedMinTools := 18
	expectedMaxTools := 25

	toolNames := server.ToolNames()
	toolCount := len(toolNames)

	if toolCount < expectedMinTools {
		t.Errorf("Expected at least %d tools, got %d: %v",
			expectedMinTools, toolCount, toolNames)
	}
	if toolCount > expectedMaxTools {
		t.Errorf("Expected at most %d tools, got %d: %v",
			expectedMaxTools, toolCount, toolNames)
	}

	t.Logf("Registered %d tools: %v", toolCount, toolNames)

	// Verify all expected tools exist
	expectedTools := []string{
		"liza_get",
		"liza_status",
		"liza_validate",
		"liza_version",
		"liza_add_task",
		"liza_claim_task",
		"liza_submit_for_review",
		"liza_handoff",
		"liza_submit_verdict",
		"liza_mark_blocked",
		"liza_release_claim",
		"liza_supersede_task",
		"liza_wt_create",
		"liza_wt_delete",
		"liza_wt_merge",
		"liza_analyze",
		"liza_update_sprint_metrics",
		"liza_checkpoint",
		"liza_clear_stale_review_claims",
		"liza_delete_agent",
	}

	for _, expected := range expectedTools {
		if _, ok := server.GetTool(expected); !ok {
			t.Errorf("Expected tool %s not found", expected)
		}
	}
}

// TestAllToolsHaveExpectations ensures every registered tool has parameter expectations defined
func TestAllToolsHaveExpectations(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	toolNames := server.ToolNames()

	var missingExpectations []string
	for _, toolName := range toolNames {
		if _, ok := expectedToolParams[toolName]; !ok {
			missingExpectations = append(missingExpectations, toolName)
		}
	}

	if len(missingExpectations) > 0 {
		t.Errorf("Tools missing parameter expectations: %v", missingExpectations)
		t.Log("Add these tools to the expectedToolParams map in schema_consistency_test.go")
	}
}

// TestSchemaPropertiesCoverage verifies schema properties match expectation keys
func TestSchemaPropertiesCoverage(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	for toolName, expectations := range expectedToolParams {
		tool, ok := server.GetTool(toolName)
		if !ok {
			t.Fatalf("Tool %s from expectations not found in server", toolName)
		}

		// Check that all expected params exist in schema properties
		for paramName := range expectations {
			if _, ok := tool.InputSchema.Properties[paramName]; !ok {
				// It's okay if required params don't have properties (they just need to be in Required)
				isRequired := slices.Contains(tool.InputSchema.Required, paramName)
				if !isRequired {
					t.Errorf("Tool %s: expected param %q not found in schema properties and not required",
						toolName, paramName)
				}
			}
		}

		// Check that all schema properties have expectations
		for propName := range tool.InputSchema.Properties {
			if _, ok := expectations[propName]; !ok {
				t.Errorf("Tool %s: schema property %q has no expectation defined", toolName, propName)
			}
		}
	}
}

// TestRequiredFieldsHaveProperties ensures all required fields are also in properties
func TestRequiredFieldsHaveProperties(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	for _, toolName := range server.ToolNames() {
		tool, ok := server.GetTool(toolName)
		if !ok {
			continue
		}

		for _, required := range tool.InputSchema.Required {
			if _, ok := tool.InputSchema.Properties[required]; !ok {
				// Required fields should typically have property definitions
				// but it's not strictly required by JSON schema
				t.Logf("Tool %s: required field %q has no property definition", toolName, required)
			}
		}
	}
}

// TestNoDuplicateToolNames ensures no duplicate tool registrations
func TestNoDuplicateToolNames(t *testing.T) {
	server := NewServer("/tmp", "/tmp/log.yaml")

	toolNames := server.ToolNames()
	seen := make(map[string]bool)

	for _, name := range toolNames {
		if seen[name] {
			t.Errorf("Tool %s is registered more than once", name)
		}
		seen[name] = true
	}
}

// extractSchemaRequired gets required fields from InputSchema
func extractSchemaRequired(schema protocol.InputSchema) []string {
	result := make([]string, len(schema.Required))
	copy(result, schema.Required)
	return result
}

// stringSlicesEqual compares two string slices for equality (order-independent)
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]bool)
	bMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	for _, v := range b {
		bMap[v] = true
	}
	for k := range aMap {
		if !bMap[k] {
			return false
		}
	}
	return true
}
