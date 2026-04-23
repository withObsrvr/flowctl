package components

import (
	"encoding/json"
	"fmt"
	"os"
)

// Metadata represents component metadata from metadata.json
type Metadata struct {
	Name         string                    `json:"name"`
	Version      string                    `json:"version"`
	Description  string                    `json:"description"`
	Type         string                    `json:"type"`          // "source", "processor", "sink"
	Produces     []string                  `json:"produces"`      // Event types produced
	Consumes     []string                  `json:"consumes"`      // Event types consumed
	InputTypes   []string                  `json:"input_types"`
	OutputTypes  []string                  `json:"output_types"`
	EnvMapping   map[string]string         `json:"env_mapping"`   // Config key -> ENV var mapping
	ConfigSchema map[string]ConfigProperty `json:"config_schema"` // Alternative metadata format used by published components
}

type ConfigProperty struct {
	Env string `json:"env"`
}

// LoadMetadata loads component metadata from a file
func LoadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	metadata.normalizeEnvMapping()
	return &metadata, nil
}

func (m *Metadata) normalizeEnvMapping() {
	if len(m.Consumes) == 0 && len(m.InputTypes) > 0 {
		m.Consumes = append([]string(nil), m.InputTypes...)
	}
	if len(m.Produces) == 0 && len(m.OutputTypes) > 0 {
		m.Produces = append([]string(nil), m.OutputTypes...)
	}
	if m.EnvMapping == nil {
		m.EnvMapping = make(map[string]string)
	}

	for key, prop := range m.ConfigSchema {
		if prop.Env != "" {
			if _, exists := m.EnvMapping[key]; !exists {
				m.EnvMapping[key] = prop.Env
			}
		}
	}
}

// SaveMetadata saves component metadata to a file
func SaveMetadata(path string, metadata *Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
