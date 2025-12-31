package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/withobsrvr/flowctl/internal/components"
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
		zap.Int("sinks", len(pipeline.Spec.Sinks)),
		zap.Int("pipelines", len(pipeline.Spec.Pipelines)))

	return &pipeline, nil
}

// ResolveComponents resolves component registry references and translates config to env vars
func ResolveComponents(ctx context.Context, pipeline *model.Pipeline, controlPlaneEndpoint string) error {
	logger.Debug("Resolving component references")

	// Create component resolver
	resolver, err := components.NewResolver("")
	if err != nil {
		return fmt.Errorf("failed to create component resolver: %w", err)
	}

	// Resolve sources
	for i := range pipeline.Spec.Sources {
		if err := resolveComponent(ctx, &pipeline.Spec.Sources[i], resolver, controlPlaneEndpoint); err != nil {
			return fmt.Errorf("failed to resolve source %s: %w", pipeline.Spec.Sources[i].ID, err)
		}
	}

	// Resolve processors
	for i := range pipeline.Spec.Processors {
		if err := resolveComponent(ctx, &pipeline.Spec.Processors[i], resolver, controlPlaneEndpoint); err != nil {
			return fmt.Errorf("failed to resolve processor %s: %w", pipeline.Spec.Processors[i].ID, err)
		}
	}

	// Resolve sinks
	for i := range pipeline.Spec.Sinks {
		if err := resolveComponent(ctx, &pipeline.Spec.Sinks[i], resolver, controlPlaneEndpoint); err != nil {
			return fmt.Errorf("failed to resolve sink %s: %w", pipeline.Spec.Sinks[i].ID, err)
		}
	}

	return nil
}

// resolveComponent resolves a single component reference
func resolveComponent(ctx context.Context, comp *model.Component, resolver *components.Resolver, controlPlaneEndpoint string) error {
	// Skip if no type reference (using explicit command or image)
	if comp.Type == "" {
		return nil
	}

	// Check if it's a registry reference (contains @)
	if !isRegistryReference(comp.Type) {
		// Just a type name, not a registry reference - skip
		return nil
	}

	// Resolve the component
	resolvedComp, err := resolver.Resolve(ctx, comp.Type)
	if err != nil {
		return err
	}

	// Set the command to the resolved binary path
	comp.Command = []string{resolvedComp.BinaryPath}

	// Translate config to env vars
	configEnv := components.TranslateConfig(comp.Config, resolvedComp.Metadata)

	// Merge with explicit env vars (explicit takes precedence)
	comp.Env = components.MergeEnv(configEnv, comp.Env)

	// Add flowctl env vars
	comp.Env = components.AddFlowctlEnv(comp.Env, comp.ID, controlPlaneEndpoint)

	logger.Debug("Resolved component",
		zap.String("id", comp.ID),
		zap.String("type", comp.Type),
		zap.String("binary", resolvedComp.BinaryPath))

	return nil
}

// isRegistryReference checks if a type string is a registry reference (contains @)
func isRegistryReference(typeStr string) bool {
	return len(typeStr) > 0 && (typeStr[0] != '/' && contains(typeStr, "@"))
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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

	// Check that we have at least one component (source or pipeline)
	if len(pipeline.Spec.Sources) == 0 && len(pipeline.Spec.Pipelines) == 0 {
		return fmt.Errorf("at least one source or pipeline component is required")
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

		// For embedded control plane, we need either command, image, or type
		if len(source.Command) == 0 && source.Image == "" && source.Type == "" {
			return fmt.Errorf("source %s must have either command, image, or type specified", source.ID)
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

		// For embedded control plane, we need either command, image, or type
		if len(processor.Command) == 0 && processor.Image == "" && processor.Type == "" {
			return fmt.Errorf("processor %s must have either command, image, or type specified", processor.ID)
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

		// For embedded control plane, we need either command, image, or type
		if len(sink.Command) == 0 && sink.Image == "" && sink.Type == "" {
			return fmt.Errorf("sink %s must have either command, image, or type specified", sink.ID)
		}

		// Validate inputs exist
		for _, input := range sink.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("sink %s references unknown input %s", sink.ID, input)
			}
		}
	}

	// Validate pipelines
	for _, pipeline := range pipeline.Spec.Pipelines {
		if pipeline.ID == "" {
			return fmt.Errorf("pipeline id is required")
		}
		if componentIDs[pipeline.ID] {
			return fmt.Errorf("duplicate component id: %s", pipeline.ID)
		}
		componentIDs[pipeline.ID] = true

		// Pipelines must have an image
		if pipeline.Image == "" {
			return fmt.Errorf("pipeline %s must have an image specified", pipeline.ID)
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