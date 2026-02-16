package mcp

import (
	"testing"
)

// TestServerInitialization verifies server starts without errors
func TestServerInitialization(t *testing.T) {
	server := NewServer("/tmp/test-project", "/tmp/test-project/.liza/log.yaml")

	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.projectRoot != "/tmp/test-project" {
		t.Errorf("Expected projectRoot /tmp/test-project, got %s", server.projectRoot)
	}

	if server.logPath != "/tmp/test-project/.liza/log.yaml" {
		t.Errorf("Expected logPath /tmp/test-project/.liza/log.yaml, got %s", server.logPath)
	}
}

// TestCapabilitiesResponse verifies server reports correct capabilities
func TestCapabilitiesResponse(t *testing.T) {
	server := NewServer("/tmp/test-project", "/tmp/test-project/.liza/log.yaml")

	caps := server.GetCapabilities()

	if caps == nil {
		t.Fatal("Expected capabilities to be returned")
	}

	// Should have tools capability
	if _, ok := caps["tools"]; !ok {
		t.Error("Expected tools capability")
	}

	// Should have resources capability
	if _, ok := caps["resources"]; !ok {
		t.Error("Expected resources capability")
	}
}

// TestToolRegistration verifies tools registered correctly
func TestToolRegistration(t *testing.T) {
	server := NewServer("/tmp/test-project", "/tmp/test-project/.liza/log.yaml")

	// Phase 1: Should have read-only tools
	expectedTools := []string{
		"liza_get",
		"liza_status",
		"liza_validate",
		"liza_version",
	}

	tools := server.ListTools()

	if len(tools) == 0 {
		t.Fatal("Expected tools to be registered")
	}

	for _, expected := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s to be registered", expected)
		}
	}
}

// TestResourceRegistration verifies resources registered correctly
func TestResourceRegistration(t *testing.T) {
	server := NewServer("/tmp/test-project", "/tmp/test-project/.liza/log.yaml")

	// Phase 1: Should have read-only resources
	expectedResources := []string{
		"liza://state",
		"liza://tasks",
		"liza://agents",
	}

	resources := server.ListResources()

	if len(resources) == 0 {
		t.Fatal("Expected resources to be registered")
	}

	for _, expected := range expectedResources {
		found := false
		for _, resource := range resources {
			if resource.URI == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected resource %s to be registered", expected)
		}
	}
}
