package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration
type Config struct {
	Version    string            `yaml:"version"`
	LogLevel   string            `yaml:"log_level"`
	Source     SourceConfig      `yaml:"source"`
	Processors []ProcessorConfig `yaml:"processors"`
	Sink       SinkConfig        `yaml:"sink"`
}

// SourceConfig represents source configuration
type SourceConfig struct {
	Type   string         `yaml:"type"`
	Params map[string]any `yaml:"params"`
}

// ProcessorConfig represents processor configuration
type ProcessorConfig struct {
	Name   string         `yaml:"name"`
	Plugin string         `yaml:"plugin"`
	Params map[string]any `yaml:"params"`
}

// SinkConfig represents sink configuration
type SinkConfig struct {
	Type   string         `yaml:"type"`
	Params map[string]any `yaml:"params"`
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	if c.Source.Type == "" {
		return fmt.Errorf("source type is required")
	}

	if len(c.Processors) == 0 {
		return fmt.Errorf("at least one processor is required")
	}

	if c.Sink.Type == "" {
		return fmt.Errorf("sink type is required")
	}

	return nil
}
