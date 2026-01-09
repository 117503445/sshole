package hub

import (
	"encoding/json"
	"fmt"
	"os"
)

// PortMapping represents the fixed agent<->port mapping loaded at startup.
type PortMapping struct {
	Agents map[string]int `json:"agents"`
}

// LoadMapping loads the mapping JSON from disk.
func LoadMapping(path string) (*PortMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load mapping: %w", err)
	}
	var pm PortMapping
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("parse mapping: %w", err)
	}
	if pm.Agents == nil {
		pm.Agents = map[string]int{}
	}
	return &pm, nil
}
