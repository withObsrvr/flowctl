package components

import (
	"encoding/json"
	"fmt"
	"os"
)

// Metadata represents component metadata from metadata.json
type Metadata struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Type       string            `json:"type"`        // "source", "processor", "sink"
	Produces   []string          `json:"produces"`    // Event types produced
	Consumes   []string          `json:"consumes"`    // Event types consumed
	EnvMapping map[string]string `json:"env_mapping"` // Config key -> ENV var mapping
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

	return &metadata, nil
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
