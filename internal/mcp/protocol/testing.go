package protocol

import (
	"encoding/json"
	"testing"
)

// mustMarshalToMap marshals v to JSON and unmarshals it into a map.
// It fails the test if either step errors.
func mustMarshalToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	return parsed
}

// mustGetMap extracts a nested map from m by key, failing if not found or wrong type.
func mustGetMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("Expected %q to be a map", key)
	}
	return v
}

// mustGetSlice extracts a nested slice from m by key, failing if not found or wrong type.
func mustGetSlice(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	v, ok := m[key].([]any)
	if !ok {
		t.Fatalf("Expected %q to be a slice", key)
	}
	return v
}
