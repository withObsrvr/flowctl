package config

import (
	"fmt"
	"os"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
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
	logger.Debug("Loading config from file", zap.String("path", path))
	
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("Failed to read config file", 
			zap.String("path", path),
			zap.Error(err))
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("Failed to parse config file", 
			zap.String("path", path),
			zap.Error(err))
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	logger.Debug("Successfully loaded config file", 
		zap.String("path", path), 
		zap.String("version", cfg.Version))
	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	logger.Debug("Validating configuration")
	
	if c.Version == "" {
		logger.Error("Invalid configuration", zap.String("reason", "missing version"))
		return fmt.Errorf("version is required")
	}

	if c.Source.Type == "" {
		logger.Error("Invalid configuration", zap.String("reason", "missing source type"))
		return fmt.Errorf("source type is required")
	}

	if len(c.Processors) == 0 {
		logger.Error("Invalid configuration", zap.String("reason", "no processors defined"))
		return fmt.Errorf("at least one processor is required")
	}

	if c.Sink.Type == "" {
		logger.Error("Invalid configuration", zap.String("reason", "missing sink type"))
		return fmt.Errorf("sink type is required")
	}

	logger.Debug("Configuration validated successfully", 
		zap.String("version", c.Version),
		zap.String("source", c.Source.Type),
		zap.Int("processors", len(c.Processors)),
		zap.String("sink", c.Sink.Type))
	return nil
}
