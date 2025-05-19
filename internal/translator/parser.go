package translator

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Parser is the interface for parsing pipeline configurations
type Parser interface {
	// Parse parses input data into a config structure
	Parse(data []byte) (*config.Config, error)

	// ParseReader parses from an io.Reader
	ParseReader(r io.Reader) (*config.Config, error)
}

// YAMLParser implements Parser for YAML files
type YAMLParser struct{}

// Parse parses YAML input data into a config structure
func (p *YAMLParser) Parse(data []byte) (*config.Config, error) {
	logger.Debug("Parsing YAML configuration")

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("Failed to parse YAML", zap.Error(err))
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &cfg, nil
}

// ParseReader parses YAML from an io.Reader
func (p *YAMLParser) ParseReader(r io.Reader) (*config.Config, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	return p.Parse(data)
}

// JSONParser implements Parser for JSON files
type JSONParser struct{}

// Parse parses JSON input data into a config structure
func (p *JSONParser) Parse(data []byte) (*config.Config, error) {
	logger.Debug("Parsing JSON configuration")

	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Error("Failed to parse JSON", zap.Error(err))
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &cfg, nil
}

// ParseReader parses JSON from an io.Reader
func (p *JSONParser) ParseReader(r io.Reader) (*config.Config, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	return p.Parse(data)
}

// CUEParser implements Parser for CUE files
// This is a placeholder for future implementation
type CUEParser struct{}

// Parse parses CUE input data into a config structure
func (p *CUEParser) Parse(data []byte) (*config.Config, error) {
	return nil, fmt.Errorf("CUE parsing not yet implemented")
}

// ParseReader parses CUE from an io.Reader
func (p *CUEParser) ParseReader(r io.Reader) (*config.Config, error) {
	return nil, fmt.Errorf("CUE parsing not yet implemented")
}