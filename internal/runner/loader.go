package runner

import (
	"fmt"
	"os"

	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// LoadPipelineFromFile loads a pipeline from a YAML file
func LoadPipelineFromFile(filePath string) (*model.Pipeline, error) {
	logger.Debug("Loading pipeline from file", zap.String("path", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file: %w", err)
	}

	var pipeline model.Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline YAML: %w", err)
	}

	// Validate the pipeline
	if err := validatePipeline(&pipeline); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	logger.Debug("Successfully loaded pipeline",
		zap.String("name", pipeline.Metadata.Name),
		zap.Int("sources", len(pipeline.Spec.Sources)),
		zap.Int("processors", len(pipeline.Spec.Processors)),
		zap.Int("sinks", len(pipeline.Spec.Sinks)))

	return &pipeline, nil
}

// validatePipeline validates the pipeline structure
func validatePipeline(pipeline *model.Pipeline) error {
	// Check required fields
	if pipeline.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if pipeline.Kind != "Pipeline" {
		return fmt.Errorf("kind must be 'Pipeline', got: %s", pipeline.Kind)
	}

	if pipeline.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	// Check that we have at least one source
	if len(pipeline.Spec.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	// Validate component IDs are unique
	componentIDs := make(map[string]bool)

	// Validate sources
	for _, source := range pipeline.Spec.Sources {
		if source.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if componentIDs[source.ID] {
			return fmt.Errorf("duplicate component id: %s", source.ID)
		}
		componentIDs[source.ID] = true

		// For embedded control plane, we need either image or type
		if source.Image == "" && source.Type == "" {
			return fmt.Errorf("source %s must have either image or type specified", source.ID)
		}
	}

	// Validate processors
	for _, processor := range pipeline.Spec.Processors {
		if processor.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if componentIDs[processor.ID] {
			return fmt.Errorf("duplicate component id: %s", processor.ID)
		}
		componentIDs[processor.ID] = true

		// For embedded control plane, we need either image or type
		if processor.Image == "" && processor.Type == "" {
			return fmt.Errorf("processor %s must have either image or type specified", processor.ID)
		}

		// Validate inputs exist
		for _, input := range processor.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("processor %s references unknown input %s", processor.ID, input)
			}
		}
	}

	// Validate sinks
	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if componentIDs[sink.ID] {
			return fmt.Errorf("duplicate component id: %s", sink.ID)
		}
		componentIDs[sink.ID] = true

		// For embedded control plane, we need either image or type
		if sink.Image == "" && sink.Type == "" {
			return fmt.Errorf("sink %s must have either image or type specified", sink.ID)
		}

		// Validate inputs exist
		for _, input := range sink.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("sink %s references unknown input %s", sink.ID, input)
			}
		}
	}

	return nil
}

// LoadPipelineFromBytes loads a pipeline from YAML bytes
func LoadPipelineFromBytes(data []byte) (*model.Pipeline, error) {
	var pipeline model.Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline YAML: %w", err)
	}

	// Validate the pipeline
	if err := validatePipeline(&pipeline); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return &pipeline, nil
}