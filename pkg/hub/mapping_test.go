package hub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMappingCreatesFileWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "port_mapping.json")
	pm, err := LoadMapping(path)
	if err != nil {
		t.Fatalf("LoadMapping: %v", err)
	}
	if pm.Agents == nil || len(pm.Agents) != 0 {
		t.Fatalf("expected empty mapping, got %+v", pm.Agents)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("mapping file not created: %v", err)
	}
}

func TestMappingRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "port_mapping.json")
	want := &PortMapping{Agents: map[string]int{"a": 10001, "b": 10002}}
	if err := SaveMapping(path, want); err != nil {
		t.Fatalf("SaveMapping: %v", err)
	}
	got, err := LoadMapping(path)
	if err != nil {
		t.Fatalf("LoadMapping: %v", err)
	}
	if got.Agents["a"] != 10001 || got.Agents["b"] != 10002 {
		t.Fatalf("round trip mismatch: %+v", got.Agents)
	}
}

func TestLoadMappingInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "port_mapping.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadMapping(path); err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestNewHubRejectsDuplicateHubPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "port_mapping.json")
	pm := &PortMapping{Agents: map[string]int{"a": 10001, "b": 10001}}
	if err := SaveMapping(path, pm); err != nil {
		t.Fatalf("SaveMapping: %v", err)
	}
	if _, err := NewHub(HubConfig{MappingFile: path}); err == nil {
		t.Fatal("expected duplicate hub port error")
	}
}
