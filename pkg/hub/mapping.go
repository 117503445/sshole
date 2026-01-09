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

// LoadMapping loads the mapping JSON from disk, creating an empty one if it doesn't exist.
func LoadMapping(path string) (*PortMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create an empty mapping file
			pm := &PortMapping{Agents: map[string]int{}}
			if err := SaveMapping(path, pm); err != nil {
				return nil, fmt.Errorf("create empty mapping: %w", err)
			}
			return pm, nil
		}
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

// SaveMapping saves the mapping JSON to disk.
func SaveMapping(path string, pm *PortMapping) error {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write mapping: %w", err)
	}
	return nil
}
