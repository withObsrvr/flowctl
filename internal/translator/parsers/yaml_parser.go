package parsers

import (
	"fmt"

	"github.com/withobsrvr/flowctl/internal/translator/models"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// YAMLParser parses pipeline YAML into a Pipeline object
type YAMLParser struct{}

// NewYAMLParser creates a new YAML parser
func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

// Parse parses YAML data into a Pipeline object
func (p *YAMLParser) Parse(data []byte) (*models.Pipeline, error) {
	var pipeline models.Pipeline
	
	err := yaml.Unmarshal(data, &pipeline)
	if err != nil {
		logger.Error("Failed to parse YAML", zap.Error(err))
		return nil, fmt.Errorf("failed to parse pipeline YAML: %w", err)
	}

	// Validate the pipeline
	if err := p.validate(&pipeline); err != nil {
		logger.Error("Pipeline validation failed", zap.Error(err))
		return nil, err
	}

	logger.Debug("Successfully parsed pipeline",
		zap.String("name", pipeline.Metadata.Name),
		zap.Int("sources", len(pipeline.Spec.Sources)),
		zap.Int("processors", len(pipeline.Spec.Processors)),
		zap.Int("sinks", len(pipeline.Spec.Sinks)))

	return &pipeline, nil
}

// validate checks if the parsed pipeline is valid
func (p *YAMLParser) validate(pipeline *models.Pipeline) error {
	// Check required fields
	if pipeline.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	
	if pipeline.Kind != "Pipeline" {
		return fmt.Errorf("kind must be 'Pipeline'")
	}
	
	if pipeline.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	
	// Check components
	if len(pipeline.Spec.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}
	
	// Validate that each component has required fields
	for _, src := range pipeline.Spec.Sources {
		if src.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if src.Image == "" {
			return fmt.Errorf("source image is required for %s", src.ID)
		}
	}
	
	for _, proc := range pipeline.Spec.Processors {
		if proc.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if proc.Image == "" {
			return fmt.Errorf("processor image is required for %s", proc.ID)
		}
		if len(proc.Inputs) == 0 {
			return fmt.Errorf("processor %s must have at least one input", proc.ID)
		}
	}
	
	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if sink.Image == "" {
			return fmt.Errorf("sink image is required for %s", sink.ID)
		}
		if len(sink.Inputs) == 0 {
			return fmt.Errorf("sink %s must have at least one input", sink.ID)
		}
	}
	
	// Validate connections between components
	// (Check that all input references exist)
	componentIDs := make(map[string]bool)
	
	// Collect all component IDs
	for _, src := range pipeline.Spec.Sources {
		componentIDs[src.ID] = true
	}
	
	// Check processor inputs
	for _, proc := range pipeline.Spec.Processors {
		componentIDs[proc.ID] = true
		for _, input := range proc.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("processor %s references unknown input %s", proc.ID, input)
			}
		}
	}
	
	// Check sink inputs
	for _, sink := range pipeline.Spec.Sinks {
		for _, input := range sink.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("sink %s references unknown input %s", sink.ID, input)
			}
		}
	}
	
	return nil
}